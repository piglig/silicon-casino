export type JoinMode = { mode: "random" } | { mode: "select"; roomId: string };

export type JoinMessage = {
  type: "join";
  agent_id: string;
  api_key: string;
  join_mode: "random" | "select";
  room_id?: string;
};

export type ActionMessage = {
  type: "action";
  request_id: string;
  action: "fold" | "check" | "call" | "raise" | "bet";
  amount?: number;
  thought_log?: string;
};

export type JoinResultEvent = {
  type: "join_result";
  protocol_version: string;
  ok: boolean;
  error?: string;
  room_id?: string;
};

export type StateUpdateEvent = {
  type: "state_update";
  protocol_version: string;
  game_id: string;
  hand_id: string;
  my_seat: number;
  current_actor_seat: number;
  min_raise: number;
  current_bet: number;
  call_amount: number;
  my_balance: number;
  action_timeout_ms: number;
  street: string;
  hole_cards?: string[];
  community_cards: string[];
  pot: number;
  opponents: Array<{
    seat: number;
    name: string;
    stack: number;
    action: string;
  }>;
};

export type ActionResultEvent = {
  type: "action_result";
  protocol_version: string;
  request_id: string;
  ok: boolean;
  error?: string;
};

export type EventLogEvent = {
  type: "event_log";
  protocol_version: string;
  timestamp_ms: number;
  player_seat: number;
  action: string;
  amount?: number;
  thought_log?: string;
  event: string;
};

export type HandEndEvent = {
  type: "hand_end";
  protocol_version: string;
  winner: string;
  pot: number;
  balances: Array<{ agent_id: string; balance: number }>;
  showdown?: Array<{ agent_id: string; hole_cards: string[] }>;
};

export type ServerEvent =
  | JoinResultEvent
  | StateUpdateEvent
  | ActionResultEvent
  | EventLogEvent
  | HandEndEvent
  | { type: string; [k: string]: unknown };
