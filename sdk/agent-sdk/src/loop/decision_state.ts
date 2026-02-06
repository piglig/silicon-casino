import { promises as fs } from "node:fs";
import path from "node:path";

export type DecisionState = {
  version?: number;
  session_id?: string;
  stream_url?: string;
  last_event_id?: string;
  last_turn_id?: string;
  updated_at?: string;
};

const STATE_VERSION = 1;

export function defaultDecisionStatePath(): string {
  return path.join(process.cwd(), "decision_state.json");
}

export async function loadDecisionState(
  filePath: string = defaultDecisionStatePath()
): Promise<DecisionState> {
  let raw = "";
  try {
    raw = await fs.readFile(filePath, "utf8");
  } catch (err) {
    if (isENOENT(err)) {
      return { version: STATE_VERSION };
    }
    throw err;
  }
  if (!raw.trim()) {
    return { version: STATE_VERSION };
  }
  try {
    const parsed = JSON.parse(raw) as DecisionState;
    if (parsed && typeof parsed === "object") {
      return { ...parsed, version: STATE_VERSION };
    }
  } catch {
    return { version: STATE_VERSION };
  }
  return { version: STATE_VERSION };
}

export async function saveDecisionState(
  state: DecisionState,
  filePath: string = defaultDecisionStatePath()
): Promise<void> {
  const record: DecisionState = {
    ...state,
    version: STATE_VERSION,
    updated_at: new Date().toISOString()
  };
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, `${JSON.stringify(record, null, 2)}\n`, { mode: 0o600 });
}

function isENOENT(err: unknown): err is NodeJS.ErrnoException {
  return Boolean(err && typeof err === "object" && (err as NodeJS.ErrnoException).code === "ENOENT");
}
