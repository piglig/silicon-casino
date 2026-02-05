import { APAHttpClient } from "./http/client.js";
import { resolveApiBase, requireArg } from "./utils/config.js";
import { DecisionCallbackServer } from "./loop/callback.js";
import { loadCredential, saveCredential } from "./loop/credentials.js";
import { TurnTracker } from "./loop/state.js";

type ArgMap = Record<string, string | boolean>;

function parseArgs(argv: string[]): { command: string; args: ArgMap } {
  const [command = "help", ...rest] = argv;
  const args: ArgMap = {};
  for (let i = 0; i < rest.length; i += 1) {
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

function readNumber(args: ArgMap, key: string, fallback?: number): number {
  const raw = args[key];
  if (raw === undefined && fallback !== undefined) {
    return fallback;
  }
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
  apa-bot claim (--claim-code <code> | --claim-url <url>) [--api-base <url>]
  apa-bot me [--api-base <url>]
  apa-bot bind-key --provider <openai|kimi> --vendor-key <key> --budget-usd <num> [--api-base <url>]
  apa-bot loop --join <random|select> [--room-id <id>]
               [--callback-addr <host:port>] [--decision-timeout-ms <ms>] [--api-base <url>]
  apa-bot doctor [--api-base <url>]

Config priority: CLI args > env (API_BASE) > defaults.`);
}

async function requireApiKey(apiBase: string): Promise<string> {
  const cached = await loadCredential(apiBase, undefined);
  if (!cached?.api_key) {
    throw new Error("api_key_not_found (run apa-bot register)");
  }
  return cached.api_key;
}

function claimCodeFromUrl(raw: string): string {
  try {
    const url = new URL(raw);
    const parts = url.pathname.split("/").filter(Boolean);
    return parts[parts.length - 1] || "";
  } catch {
    return "";
  }
}

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

function pickRoom(
  rooms: Array<{ id: string; min_buyin_cc: number; name?: string }>,
  joinMode: "random" | "select",
  roomId?: string
): { id: string; min_buyin_cc: number } {
  if (rooms.length === 0) {
    throw new Error("no_rooms_available");
  }
  if (joinMode === "select") {
    const match = rooms.find((room) => room.id === roomId);
    if (!match) {
      throw new Error("room_not_found");
    }
    return { id: match.id, min_buyin_cc: match.min_buyin_cc };
  }
  const sorted = [...rooms].sort((a, b) => a.min_buyin_cc - b.min_buyin_cc);
  return { id: sorted[0].id, min_buyin_cc: sorted[0].min_buyin_cc };
}

async function runLoop(args: ArgMap): Promise<void> {
  const apiBase = resolveApiBase(readString(args, "api-base", "API_BASE"));
  const joinRaw = requireArg("--join", readString(args, "join"));
  const joinMode = joinRaw === "select" ? "select" : "random";
  const roomId = joinMode === "select" ? requireArg("--room-id", readString(args, "room-id")) : undefined;
  const callbackAddr = readString(args, "callback-addr");
  const decisionTimeoutMs = readNumber(args, "decision-timeout-ms", 5000);

  const client = new APAHttpClient({ apiBase });
  const cached = await loadCredential(apiBase, undefined);
  if (!cached) {
    throw new Error("credential_not_found (run apa-bot register first)");
  }
  const agentId = cached.agent_id;
  const apiKey = cached.api_key;

  const meStatus = await client.getAgentMe(apiKey);
  emit({ type: "agent_status", status: meStatus });
  if (meStatus?.status === "pending") {
    emit({ type: "claim_required", message: "agent is pending; complete claim before starting loop" });
    return;
  }

  let me = await client.getAgentMe(apiKey);
  let balance = Number(me?.balance_cc ?? 0);

  const rooms = await client.listPublicRooms();
  const pickedRoom = pickRoom(rooms.items || [], joinMode, roomId);

  if (balance < pickedRoom.min_buyin_cc) {
    throw new Error(`insufficient_balance (balance=${balance}, min=${pickedRoom.min_buyin_cc})`);
  }

  const callbackServer = new DecisionCallbackServer(callbackAddr);
  const callbackURL = await callbackServer.start();

  const session = await client.createSession({
    agentID: agentId,
    apiKey,
    joinMode: "select",
    roomID: pickedRoom.id
  });
  const sessionId = session.session_id as string;
  const streamURL = String(session.stream_url || "");
  const base = apiBase.replace(/\/api\/?$/, "");
  const resolvedStreamURL = streamURL.startsWith("http") ? streamURL : `${base}${streamURL}`;

  emit({
    type: "ready",
    agent_id: agentId,
    session_id: sessionId,
    stream_url: resolvedStreamURL,
    callback_url: callbackURL
  });

  let lastEventId = "";
  let stopRequested = false;
  const tracker = new TurnTracker();

  const stop = async (): Promise<void> => {
    if (stopRequested) return;
    stopRequested = true;
    await callbackServer.stop();
    await client.closeSession(sessionId);
    emit({ type: "stopped", session_id: sessionId });
  };

  process.on("SIGINT", () => {
    emit({ type: "signal", signal: "SIGINT" });
    void stop();
  });

  while (!stopRequested) {
    try {
      lastEventId = await parseSSE(resolvedStreamURL, lastEventId, async (evt) => {
        let envelope: any;
        try {
          envelope = JSON.parse(evt.data);
        } catch {
          return;
        }
        const evType = envelope?.event || evt.event;
        const data = envelope?.data || {};
        emit({ type: "server_event", event: evType, event_id: evt.id || "" });

        if (evType === "session_closed") {
          await stop();
          return;
        }

        if (evType !== "state_snapshot") {
          return;
        }

        if (!tracker.shouldRequestDecision(data)) {
          return;
        }

        const reqID = `req_${Date.now()}_${Math.floor(Math.random() * 1_000_000_000)}`;
        emit({
          type: "decision_request",
          request_id: reqID,
          session_id: sessionId,
          turn_id: data.turn_id,
          callback_url: callbackURL,
          legal_actions: ["fold", "check", "call", "raise", "bet"],
          state: data
        });

        try {
          const decision = await callbackServer.waitForDecision(reqID, decisionTimeoutMs);
          const result = await client.submitAction({
            sessionID: sessionId,
            requestID: reqID,
            turnID: data.turn_id,
            action: decision.action,
            amount: decision.amount,
            thoughtLog: decision.thought_log
          });
          emit({ type: "action_result", request_id: reqID, ok: true, body: result });
        } catch (err) {
          emit({
            type: "decision_timeout",
            request_id: reqID,
            error: err instanceof Error ? err.message : String(err)
          });
        }
      });
    } catch (err) {
      emit({
        type: "stream_error",
        error: err instanceof Error ? err.message : String(err)
      });
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  }
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
    case "claim": {
      const client = new APAHttpClient({ apiBase });
      const claimCode = readString(args, "claim-code");
      const claimURL = readString(args, "claim-url");
      const code = claimCode || (claimURL ? claimCodeFromUrl(claimURL) : "");
      if (!code) {
        throw new Error("claim_code_required (--claim-code or --claim-url)");
      }
      const result = await client.claimByCode(code);
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "me": {
      const client = new APAHttpClient({ apiBase });
      const apiKey = await requireApiKey(apiBase);
      const result = await client.getAgentMe(apiKey);
      console.log(JSON.stringify(result, null, 2));
      return;
    }
    case "bind-key": {
      const client = new APAHttpClient({ apiBase });
      const apiKey = await requireApiKey(apiBase);
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
    case "loop": {
      await runLoop(args);
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
