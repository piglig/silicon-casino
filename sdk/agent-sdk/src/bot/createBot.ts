import { EventEmitter } from "node:events";

import { APAWsClient } from "../ws/client.js";
import type { CreateBotOptions, PlayContext, StrategyFn } from "../types/bot.js";
import type { ActionResultEvent, EventLogEvent, HandEndEvent, JoinResultEvent, StateUpdateEvent } from "../types/messages.js";
import { nextRequestId, validateBotAction } from "../utils/action.js";

const DEFAULT_GUARD_MS = 300;

type BotEvents = {
  join: JoinResultEvent;
  handEnd: HandEndEvent;
  error: unknown;
  eventLog: EventLogEvent;
};

function withTimeout<T>(promise: Promise<T>, timeoutMs: number): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("strategy_timeout")), timeoutMs);
    promise
      .then((value) => {
        clearTimeout(timer);
        resolve(value);
      })
      .catch((err) => {
        clearTimeout(timer);
        reject(err);
      });
  });
}

function buildTurnKey(s: StateUpdateEvent): string {
  return `${s.hand_id}:${s.street}:${s.current_actor_seat}:${s.current_bet}:${s.call_amount}:${s.pot}`;
}

export function createBot(opts: CreateBotOptions) {
  const emitter = new EventEmitter();
  const ws = new APAWsClient({
    apiBase: opts.apiBase,
    wsUrl: opts.wsUrl,
    agentId: opts.agentId,
    apiKey: opts.apiKey,
    join: opts.join,
    reconnect: opts.reconnect
  });

  let running = false;
  let lastTurnKey = "";

  ws.on("join_result", (evt: JoinResultEvent) => {
    if (!evt.ok) {
      emitter.emit("error", new Error(`join failed: ${evt.error || "unknown"}`));
      return;
    }
    emitter.emit("join", evt);
  });
  ws.on("event_log", (evt: EventLogEvent) => emitter.emit("eventLog", evt));
  ws.on("hand_end", (evt: HandEndEvent) => {
    lastTurnKey = "";
    emitter.emit("handEnd", evt);
  });
  ws.on("error", (err: unknown) => emitter.emit("error", err));

  async function play(strategy: StrategyFn): Promise<void> {
    running = true;
    await ws.connect();

    ws.on("state_update", async (state: StateUpdateEvent) => {
      if (!running) {
        return;
      }
      if (state.current_actor_seat !== state.my_seat) {
        return;
      }
      const turnKey = buildTurnKey(state);
      if (turnKey === lastTurnKey) {
        return;
      }
      lastTurnKey = turnKey;

      const ctx: PlayContext = {
        gameId: state.game_id,
        handId: state.hand_id,
        mySeat: state.my_seat,
        currentActorSeat: state.current_actor_seat,
        minRaise: state.min_raise,
        currentBet: state.current_bet,
        callAmount: state.call_amount,
        myBalance: state.my_balance,
        communityCards: state.community_cards,
        holeCards: state.hole_cards || [],
        raw: state
      };

      const safetyMs = Math.max(100, state.action_timeout_ms - (opts.actionTimeoutGuardMs ?? DEFAULT_GUARD_MS));
      try {
        const action = await withTimeout(Promise.resolve(strategy(ctx)), safetyMs);
        validateBotAction(action);
        await ws.sendAction({
          type: "action",
          request_id: nextRequestId(),
          action: action.action,
          amount: "amount" in action ? action.amount : undefined
        });
      } catch {
        await ws.sendAction({
          type: "action",
          request_id: nextRequestId(),
          action: "fold"
        });
      }
    });

    ws.on("action_result", (res: ActionResultEvent) => {
      if (!res.ok) {
        emitter.emit("error", new Error(`action failed: ${res.error || "unknown"} (${res.request_id})`));
      }
    });
  }

  async function stop(): Promise<void> {
    running = false;
    await ws.stop();
  }

  function on<K extends keyof BotEvents>(event: K, cb: (payload: BotEvents[K]) => void): void {
    emitter.on(event, cb);
  }

  return { play, stop, on };
}
