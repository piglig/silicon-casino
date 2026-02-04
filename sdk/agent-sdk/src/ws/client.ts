import { EventEmitter } from "node:events";

import type { ActionMessage, JoinMessage, JoinMode, ServerEvent } from "../types/messages.js";
import { computeBackoffMs, sleep } from "../utils/backoff.js";
import { resolveWsUrl } from "../utils/config.js";

type WsOptions = {
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

export class APAWsClient extends EventEmitter {
  private readonly wsUrl: string;
  private readonly agentId: string;
  private readonly apiKey: string;
  private readonly join: JoinMode;
  private readonly reconnect: Required<NonNullable<WsOptions["reconnect"]>>;
  private socket: WebSocket | null = null;
  private shouldRun = true;
  private connectAttempt = 0;

  constructor(opts: WsOptions) {
    super();
    this.wsUrl = resolveWsUrl(opts.wsUrl);
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
    await this.openSocket();
    await this.sendJoin();
  }

  async stop(): Promise<void> {
    this.shouldRun = false;
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }

  async sendAction(action: ActionMessage): Promise<void> {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error("ws not connected");
    }
    this.socket.send(JSON.stringify(action));
  }

  private async openSocket(): Promise<void> {
    const Impl: typeof WebSocket | undefined = globalThis.WebSocket;
    if (!Impl) {
      throw new Error("WebSocket is unavailable in current Node runtime (requires Node 20+)");
    }
    await new Promise<void>((resolve, reject) => {
      const ws = new Impl(this.wsUrl);
      this.socket = ws;

      ws.onopen = () => {
        this.connectAttempt = 0;
        this.emit("connected");
        resolve();
      };

      ws.onmessage = (evt) => {
        try {
          const data = JSON.parse(String(evt.data)) as ServerEvent;
          this.emit("event", data);
          this.emit(data.type, data);
        } catch (err) {
          this.emit("error", err);
        }
      };

      ws.onerror = () => {
        reject(new Error("ws connection failed"));
      };

      ws.onclose = async () => {
        this.emit("disconnected");
        if (!this.shouldRun || !this.reconnect.enabled) {
          return;
        }
        await this.reconnectLoop();
      };
    });
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
        await this.openSocket();
        await this.sendJoin();
        return;
      } catch (err) {
        this.emit("error", err);
      }
    }
  }

  private async sendJoin(): Promise<void> {
    const msg: JoinMessage = {
      type: "join",
      agent_id: this.agentId,
      api_key: this.apiKey,
      join_mode: this.join.mode
    };
    if (this.join.mode === "select") {
      msg.room_id = this.join.roomId;
    }
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error("ws not connected for join");
    }
    this.socket.send(JSON.stringify(msg));
  }
}
