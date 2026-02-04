import { APAHttpClient } from "./http/client.js";
import { createBot } from "./bot/createBot.js";
import { resolveApiBase, resolveWsUrl, requireArg } from "./utils/config.js";

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
  apa-bot play --agent-id <id> --api-key <key> --join <random|select> [--room-id <id>] [--ws-url <url>]
  apa-bot doctor [--api-base <url>] [--ws-url <url>]

Config priority: CLI args > env (API_BASE, WS_URL) > defaults.`);
}

function defaultStrategy(ctx: {
  callAmount: number;
  minRaise: number;
  currentBet: number;
}) {
  if (ctx.callAmount === 0) {
    return { action: "check" as const };
  }
  if (Math.random() < 0.75) {
    return { action: "call" as const };
  }
  if (Math.random() < 0.5) {
    return { action: "raise" as const, amount: ctx.currentBet + ctx.minRaise };
  }
  return { action: "fold" as const };
}

async function run(): Promise<void> {
  const { command, args } = parseArgs(process.argv.slice(2));

  if (command === "help" || command === "--help" || command === "-h") {
    printHelp();
    return;
  }

  const apiBase = resolveApiBase(readString(args, "api-base", "API_BASE"));
  const wsUrl = resolveWsUrl(readString(args, "ws-url", "WS_URL"));

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
    case "play": {
      const agentId = requireArg("--agent-id", readString(args, "agent-id", "AGENT_ID"));
      const apiKey = requireArg("--api-key", readString(args, "api-key", "APA_API_KEY"));
      const joinRaw = requireArg("--join", readString(args, "join"));
      const join = joinRaw === "select"
        ? { mode: "select" as const, roomId: requireArg("--room-id", readString(args, "room-id")) }
        : { mode: "random" as const };

      const bot = createBot({
        apiBase,
        agentId,
        apiKey,
        wsUrl,
        join
      });

      bot.on("join", (evt) => {
        console.log(`joined room ${evt.room_id || "unknown"}`);
      });
      bot.on("handEnd", (evt) => {
        console.log(`hand_end winner=${evt.winner} pot=${evt.pot}`);
      });
      bot.on("eventLog", (evt) => {
        console.log(`event seat=${evt.player_seat} action=${evt.action}`);
      });
      bot.on("error", (err) => {
        console.error(err instanceof Error ? err.message : String(err));
      });

      await bot.play((ctx) => defaultStrategy(ctx));
      return;
    }
    case "doctor": {
      const major = Number(process.versions.node.split(".")[0]);
      const client = new APAHttpClient({ apiBase });
      const report: Record<string, unknown> = {
        node: process.versions.node,
        node_ok: major >= 20,
        api_base: apiBase,
        ws_url: wsUrl
      };
      try {
        report.healthz = await client.healthz();
      } catch (err) {
        report.healthz_error = err instanceof Error ? err.message : String(err);
      }
      try {
        const ws = new WebSocket(wsUrl);
        await new Promise<void>((resolve, reject) => {
          ws.onopen = () => {
            ws.close();
            resolve();
          };
          ws.onerror = () => reject(new Error("ws connect failed"));
        });
        report.ws_connect = "ok";
      } catch (err) {
        report.ws_connect_error = err instanceof Error ? err.message : String(err);
      }
      console.log(JSON.stringify(report, null, 2));
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
