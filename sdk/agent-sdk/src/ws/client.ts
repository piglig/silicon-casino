import { EventEmitter } from "node:events";

import type { ActionMessage, JoinMode, ServerEvent } from "../types/messages.js";
import { computeBackoffMs, sleep } from "../utils/backoff.js";
import { resolveApiBase } from "../utils/config.js";

type WsOptions = {
  apiBase?: string;
  wsUrl?: string;
  agentId: string;
  apiKey: string;
  join: JoinMode;
  reconnect?: {
    enabled?: boolean;
    baseMs?: number;
    maxMs?: number;
    jitter?: boolean;
  };
};

const DEFAULT_RECONNECT = {
  enabled: true,
  baseMs: 500,
  maxMs: 8000,
  jitter: true
};

function apiOrigin(apiBase: string): string {
  return apiBase.replace(/\/api\/?$/, "");
}

export class APAWsClient extends EventEmitter {
  private readonly apiBase: string;
  private readonly agentId: string;
  private readonly apiKey: string;
  private readonly join: JoinMode;
  private readonly reconnect: Required<NonNullable<WsOptions["reconnect"]>>;
  private shouldRun = true;
  private connectAttempt = 0;
  private sessionId = "";
  private tableId = "";
  private roomId = "";
  private turnId = "";
  private streamAbort: AbortController | null = null;

  constructor(opts: WsOptions) {
    super();
    this.apiBase = resolveApiBase(opts.apiBase);
    this.agentId = opts.agentId;
    this.apiKey = opts.apiKey;
    this.join = opts.join;
    this.reconnect = {
      enabled: opts.reconnect?.enabled ?? DEFAULT_RECONNECT.enabled,
      baseMs: opts.reconnect?.baseMs ?? DEFAULT_RECONNECT.baseMs,
      maxMs: opts.reconnect?.maxMs ?? DEFAULT_RECONNECT.maxMs,
      jitter: opts.reconnect?.jitter ?? DEFAULT_RECONNECT.jitter
    };
  }

  async connect(): Promise<void> {
    this.shouldRun = true;
    await this.openSessionAndStream();
  }

  async stop(): Promise<void> {
    this.shouldRun = false;
    if (this.streamAbort) {
      this.streamAbort.abort();
      this.streamAbort = null;
    }
    if (this.sessionId) {
      try {
        await fetch(`${this.apiBase}/agent/sessions/${this.sessionId}`, { method: "DELETE" });
      } catch {
        // ignore best-effort close
      }
    }
  }

  async sendAction(action: ActionMessage): Promise<void> {
    if (!this.sessionId) {
      throw new Error("session not created");
    }
    if (!this.turnId) {
      throw new Error("turn_id unavailable");
    }
    const res = await fetch(`${this.apiBase}/agent/sessions/${this.sessionId}/actions`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        request_id: action.request_id,
        turn_id: this.turnId,
        action: action.action,
        amount: action.amount,
        thought_log: action.thought_log
      })
    });
    const payload = await res.json().catch(() => ({}));
    const evt = {
      type: "action_result",
      protocol_version: "",
      request_id: action.request_id,
      ok: res.ok && payload.accepted !== false,
      error: payload.reason
    };
    this.emit("action_result", evt);
  }

  private async openSessionAndStream(): Promise<void> {
    const createRes = await fetch(`${this.apiBase}/agent/sessions`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        agent_id: this.agentId,
        api_key: this.apiKey,
        join_mode: this.join.mode,
        room_id: this.join.mode === "select" ? this.join.roomId : undefined
      })
    });
    if (!createRes.ok) {
      throw new Error(`create session failed (${createRes.status})`);
    }
    const created = await createRes.json();
    this.sessionId = created.session_id;
    this.tableId = created.table_id || "";
    this.roomId = created.room_id || "";
    this.connectAttempt = 0;
    this.emit("connected");
    this.emit("join_result", {
      type: "join_result",
      protocol_version: "",
      ok: true,
      room_id: this.roomId
    });
    await this.readEventStream(created.stream_url);
  }

  private async readEventStream(streamPath: string): Promise<void> {
    const origin = apiOrigin(this.apiBase);
    const url = streamPath.startsWith("http") ? streamPath : `${origin}${streamPath}`;
    this.streamAbort = new AbortController();
    try {
      const res = await fetch(url, {
        method: "GET",
        headers: { accept: "text/event-stream" },
        signal: this.streamAbort.signal
      });
      if (!res.ok || !res.body) {
        throw new Error(`stream open failed (${res.status})`);
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let currentData = "";
      while (this.shouldRun) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";
        for (const rawLine of lines) {
          const line = rawLine.trimEnd();
          if (line.startsWith("data: ")) {
            currentData = line.slice(6);
            continue;
          }
          if (line === "") {
            if (!currentData) {
              continue;
            }
            this.handleEnvelope(currentData);
            currentData = "";
          }
        }
      }
      this.emit("disconnected");
      if (this.shouldRun && this.reconnect.enabled) {
        await this.reconnectLoop();
      }
    } catch (err) {
      if (!this.shouldRun) {
        return;
      }
      this.emit("error", err);
      if (this.reconnect.enabled) {
        await this.reconnectLoop();
      }
    }
  }

  private handleEnvelope(payloadText: string): void {
    let envelope: any;
    try {
      envelope = JSON.parse(payloadText);
    } catch (err) {
      this.emit("error", err);
      return;
    }
    const evt = envelope?.event;
    const data = envelope?.data || {};
    if (evt === "turn_started") {
      this.turnId = data.turn_id || "";
      return;
    }
    if (evt === "state_snapshot") {
      this.turnId = data.turn_id || this.turnId;
      const seats = Array.isArray(data.seats) ? data.seats : [];
      const me = seats.find((s: any) => s.seat_id === data.my_seat) || {};
      const currentBet = seats.reduce((m: number, s: any) => Math.max(m, Number(s.street_contribution || 0)), 0);
      const serverEvt: ServerEvent = {
        type: "state_update",
        protocol_version: "",
        game_id: this.tableId,
        hand_id: data.hand_id || "",
        my_seat: data.my_seat ?? 0,
        current_actor_seat: data.current_actor_seat ?? 0,
        min_raise: 0,
        current_bet: currentBet,
        call_amount: Number(me.to_call || 0),
        my_balance: Number(data.my_balance || 0),
        action_timeout_ms: Number(data.action_timeout_ms || 5000),
        street: data.street || "preflop",
        hole_cards: data.my_hole_cards || [],
        community_cards: data.community_cards || [],
        pot: Number(data.pot || 0),
        opponents: seats
          .filter((s: any) => s.seat_id !== data.my_seat)
          .map((s: any) => ({
            seat: Number(s.seat_id),
            name: String(s.agent_id || ""),
            stack: Number(s.stack || 0),
            action: String(s.last_action || "")
          }))
      } as ServerEvent;
      this.emit("event", serverEvt);
      this.emit("state_update", serverEvt);
      return;
    }
    if (evt === "action_accepted") {
      const serverEvt: ServerEvent = {
        type: "action_result",
        protocol_version: "",
        request_id: data.request_id || "",
        ok: true
      } as ServerEvent;
      this.emit("event", serverEvt);
      this.emit("action_result", serverEvt);
      return;
    }
    if (evt === "action_rejected") {
      const serverEvt: ServerEvent = {
        type: "action_result",
        protocol_version: "",
        request_id: data.request_id || "",
        ok: false,
        error: data.reason || "invalid_action"
      } as ServerEvent;
      this.emit("event", serverEvt);
      this.emit("action_result", serverEvt);
      return;
    }
    if (evt === "hand_end") {
      const serverEvt: ServerEvent = {
        type: "hand_end",
        protocol_version: "",
        winner: data.winner || "",
        pot: Number(data.pot || 0),
        balances: data.balances || [],
        showdown: data.showdown || []
      } as ServerEvent;
      this.emit("event", serverEvt);
      this.emit("hand_end", serverEvt);
      return;
    }
    if (evt === "session_closed") {
      this.emit("disconnected");
      return;
    }
  }

  private async reconnectLoop(): Promise<void> {
    while (this.shouldRun) {
      this.connectAttempt += 1;
      const waitMs = computeBackoffMs(
        this.connectAttempt,
        this.reconnect.baseMs,
        this.reconnect.maxMs,
        this.reconnect.jitter
      );
      this.emit("reconnect", { attempt: this.connectAttempt, waitMs });
      await sleep(waitMs);
      try {
        await this.openSessionAndStream();
        return;
      } catch (err) {
        this.emit("error", err);
      }
    }
  }
}
