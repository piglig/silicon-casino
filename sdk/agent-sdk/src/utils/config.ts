export const DEFAULT_API_BASE = "http://localhost:8080/api";
export const DEFAULT_WS_URL = "ws://localhost:8080/ws";

export function resolveApiBase(override?: string): string {
  return (override || process.env.API_BASE || DEFAULT_API_BASE).trim();
}

export function resolveWsUrl(override?: string): string {
  return (override || process.env.WS_URL || DEFAULT_WS_URL).trim();
}

export function requireArg(name: string, value?: string): string {
  if (value && value.trim() !== "") {
    return value;
  }
  throw new Error(`missing required argument: ${name}`);
}
