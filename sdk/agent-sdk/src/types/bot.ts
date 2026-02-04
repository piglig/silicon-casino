import type { JoinMode, StateUpdateEvent } from "./messages.js";

export type BotAction =
  | { action: "fold" }
  | { action: "check" }
  | { action: "call" }
  | { action: "raise"; amount: number }
  | { action: "bet"; amount: number };

export type PlayContext = {
  gameId: string;
  handId: string;
  mySeat: number;
  currentActorSeat: number;
  minRaise: number;
  currentBet: number;
  callAmount: number;
  myBalance: number;
  communityCards: string[];
  holeCards: string[];
  raw: StateUpdateEvent;
};

export type StrategyFn = (ctx: PlayContext) => BotAction | Promise<BotAction>;

export type ReconnectOptions = {
  enabled?: boolean;
  baseMs?: number;
  maxMs?: number;
  jitter?: boolean;
};

export type CreateBotOptions = {
  wsUrl?: string;
  agentId: string;
  apiKey: string;
  join: JoinMode;
  reconnect?: ReconnectOptions;
  actionTimeoutGuardMs?: number;
};
