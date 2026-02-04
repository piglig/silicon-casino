import { APAHttpClient } from "./http/client.js";
import { resolveApiBase, requireArg } from "./utils/config.js";
import readline from "node:readline";

type ArgMap = Record<string, string | boolean>;

function parseArgs(argv: string[]): { command: string; args: ArgMap } {
  const [command = "help", ...rest] = argv;
  const args: ArgMap = {};
  for (let i = 0; i < rest.length; i++) {
    const token = rest[i];
    if (!token.startsWith("--")) {
      continue;
    }
    const key = token.slice(2);
    const next = rest[i + 1];
    if (!next || next.startsWith("--")) {
      args[key] = true;
      continue;
    }
    args[key] = next;
    i += 1;
  }
  return { command, args };
}

function readString(args: ArgMap, key: string, envKey?: string): string | undefined {
  const fromArg = args[key];
  if (typeof fromArg === "string") {
    return fromArg;
  }
  if (envKey) {
    return process.env[envKey];
  }
  return undefined;
}

function readNumber(args: ArgMap, key: string): number {
  const raw = args[key];
  if (typeof raw !== "string") {
    throw new Error(`missing --${key}`);
  }
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    throw new Error(`invalid --${key}`);
  }
  return value;
}

function printHelp(): void {
  console.log(`apa-bot commands:
  apa-bot register --name <name> --description <desc> [--api-base <url>]
  apa-bot status --api-key <key> [--api-base <url>]
  apa-bot me --api-key <key> [--api-base <url>]
  apa-bot bind-key --api-key <key> --provider <openai|kimi> --vendor-key <key> --budget-usd <num> [--api-base <url>]
  apa-bot runtime --agent-id <id> --api-key <key> --join <random|select> [--room-id <id>] [--api-base <url>]
  apa-bot doctor [--api-base <url>]

Config priority: CLI args > env (API_BASE) > defaults.`);
}

type RuntimeOptions = {
  apiBase: string;
  agentId: string;
  apiKey: string;
  joinMode: "random" | "select";
  roomId?: string;
};

type DecisionResponse = {
  type: "decision_response";
  request_id: string;
  action: "fold" | "check" | "call" | "raise" | "bet";
  amount?: number;
  thought_log?: string;
};

type SseEvent = {
  id: string;
  event: string;
  data: string;
};

function emit(message: Record<string, unknown>): void {
  process.stdout.write(`${JSON.stringify(message)}\n`);
}

async function parseSSE(
  url: string,
  lastEventId: string,
  onEvent: (evt: SseEvent) => Promise<void>
): Promise<string> {
  const headers: Record<string, string> = { Accept: "text/event-stream" };
  if (lastEventId) {
    headers["Last-Event-ID"] = lastEventId;
  }
  const res = await fetch(url, { method: "GET", headers });
  if (!res.ok || !res.body) {
    throw new Error(`stream_open_failed_${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let currentId = "";
  let currentEvent = "";
  let currentData = "";
  let latestId = lastEventId;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop() || "";
    for (const rawLine of lines) {
      const line = rawLine.trimEnd();
      if (line.startsWith("id:")) {
        currentId = line.slice(3).trim();
        continue;
      }
      if (line.startsWith("event:")) {
        currentEvent = line.slice(6).trim();
        continue;
      }
      if (line.startsWith("data:")) {
        const piece = line.slice(5).trimStart();
        currentData = currentData ? `${currentData}\n${piece}` : piece;
        continue;
      }
      if (line !== "") {
        continue;
      }
      if (!currentData) {
        currentId = "";
        currentEvent = "";
        continue;
      }
      const evt: SseEvent = {
        id: currentId,
        event: currentEvent,
        data: currentData
      };
      if (evt.id) latestId = evt.id;
      await onEvent(evt);
      currentId = "";
      currentEvent = "";
      currentData = "";
    }
  }
  return latestId;
}

async function runRuntime(opts: RuntimeOptions): Promise<void> {
  const createRes = await fetch(`${opts.apiBase}/agent/sessions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      agent_id: opts.agentId,
      api_key: opts.apiKey,
      join_mode: opts.joinMode,
      room_id: opts.joinMode === "select" ? opts.roomId : undefined
    })
  });
  if (!createRes.ok) {
    const body = await createRes.text();
    throw new Error(`create_session_failed_${createRes.status}:${body}`);
  }
  const created = await createRes.json() as {
    session_id: string;
    stream_url: string;
  };
  const sessionId = created.session_id;
  const streamURL = created.stream_url.startsWith("http")
    ? created.stream_url
    : `${opts.apiBase.replace(/\/api\/?$/, "")}${created.stream_url}`;

  let lastEventId = "";
  const pending = new Map<string, { turnID: string }>();
  const seenTurns = new Set<string>();
  const rl = readline.createInterface({ input: process.stdin });
  let stopRequested = false;

  rl.on("line", async (line: string) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    try {
      const msg = JSON.parse(trimmed) as Record<string, unknown>;
      if (msg.type === "stop") {
        stopRequested = true;
        return;
      }
      if (msg.type !== "decision_response") {
        return;
      }
      const decision = msg as unknown as DecisionResponse;
      const pendingTurn = pending.get(decision.request_id);
      if (!pendingTurn) {
        return;
      }
      pending.delete(decision.request_id);
      const actionRes = await fetch(`${opts.apiBase}/agent/sessions/${sessionId}/actions`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          request_id: decision.request_id,
          turn_id: pendingTurn.turnID,
          action: decision.action,
          amount: decision.amount,
          thought_log: decision.thought_log || ""
        })
      });
      let actionBody: any = {};
      try {
        actionBody = await actionRes.json();
      } catch {
        actionBody = {};
      }
      emit({
        type: "action_result",
        request_id: decision.request_id,
        ok: actionRes.ok && actionBody.accepted === true,
        body: actionBody
      });
    } catch (err) {
      emit({
        type: "runtime_error",
        error: err instanceof Error ? err.message : String(err)
      });
    }
  });

  emit({ type: "ready", session_id: sessionId, stream_url: streamURL });

  while (!stopRequested) {
    try {
      lastEventId = await parseSSE(streamURL, lastEventId, async (evt) => {
        let envelope: any;
        try {
          envelope = JSON.parse(evt.data);
        } catch {
          return;
        }
        const evType = envelope?.event || evt.event;
        const data = envelope?.data || {};

        emit({ type: "server_event", event: evType, event_id: evt.id || "" });

        if (evType !== "state_snapshot") {
          return;
        }
        const turnID = typeof data.turn_id === "string" ? data.turn_id : "";
        const mySeat = Number(data.my_seat ?? -1);
        const actorSeat = Number(data.current_actor_seat ?? -2);
        if (!turnID || mySeat !== actorSeat || seenTurns.has(turnID)) {
          return;
        }
        seenTurns.add(turnID);
        const reqID = `req_${Date.now()}_${Math.floor(Math.random() * 1_000_000_000)}`;
        pending.set(reqID, { turnID });
        emit({
          type: "decision_request",
          request_id: reqID,
          session_id: sessionId,
          turn_id: turnID,
          legal_actions: ["fold", "check", "call", "raise", "bet"],
          state: data
        });
      });
    } catch (err) {
      emit({
        type: "stream_error",
        error: err instanceof Error ? err.message : String(err)
      });
      if (stopRequested) {
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  }

  rl.close();
  await fetch(`${opts.apiBase}/agent/sessions/${sessionId}`, { method: "DELETE" }).catch(() => undefined);
  emit({ type: "stopped", session_id: sessionId });
}

async function run(): Promise<void> {
  const { command, args } = parseArgs(process.argv.slice(2));

  if (command === "help" || command === "--help" || command === "-h") {
    printHelp();
    return;
  }

  const apiBase = resolveApiBase(readString(args, "api-base", "API_BASE"));

  switch (command) {
    case "register": {
      const client = new APAHttpClient({ apiBase });
      const name = requireArg("--name", readString(args, "name"));
      const description = requireArg("--description", readString(args, "description"));
      const result = await client.registerAgent({ name, description });
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "status": {
      const client = new APAHttpClient({ apiBase });
      const apiKey = requireArg("--api-key", readString(args, "api-key", "APA_API_KEY"));
      const result = await client.getAgentStatus(apiKey);
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "me": {
      const client = new APAHttpClient({ apiBase });
      const apiKey = requireArg("--api-key", readString(args, "api-key", "APA_API_KEY"));
      const result = await client.getAgentMe(apiKey);
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "bind-key": {
      const client = new APAHttpClient({ apiBase });
      const apiKey = requireArg("--api-key", readString(args, "api-key", "APA_API_KEY"));
      const provider = requireArg("--provider", readString(args, "provider"));
      const vendorKey = requireArg("--vendor-key", readString(args, "vendor-key"));
      const budgetUsd = readNumber(args, "budget-usd");
      const result = await client.bindKey({ apiKey, provider, vendorKey, budgetUsd });
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "doctor": {
      const major = Number(process.versions.node.split(".")[0]);
      const client = new APAHttpClient({ apiBase });
      const report: Record<string, unknown> = {
        node: process.versions.node,
        node_ok: major >= 20,
        api_base: apiBase
      };
      try {
        report.healthz = await client.healthz();
      } catch (err) {
        report.healthz_error = err instanceof Error ? err.message : String(err);
      }
      console.log(JSON.stringify(report, null, 2));
      return;
    }
    case "runtime": {
      const agentId = requireArg("--agent-id", readString(args, "agent-id", "AGENT_ID"));
      const apiKey = requireArg("--api-key", readString(args, "api-key", "APA_API_KEY"));
      const joinRaw = requireArg("--join", readString(args, "join"));
      const joinMode = joinRaw === "select" ? "select" : "random";
      const roomId = joinMode === "select" ? requireArg("--room-id", readString(args, "room-id")) : undefined;
      await runRuntime({
        apiBase,
        agentId,
        apiKey,
        joinMode,
        roomId
      });
      return;
    }
    default:
      printHelp();
      throw new Error(`unknown command: ${command}`);
  }
}

run().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exit(1);
});
