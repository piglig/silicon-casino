import type { AgentCredential } from "../loop/credentials.js";

export function buildCredentialFromRegisterResult(
  result: unknown,
  apiBase: string,
  fallbackName: string
): Omit<AgentCredential, "updated_at"> | null {
  const obj = (result && typeof result === "object" ? result : null) as Record<string, unknown> | null;
  if (!obj) {
    return null;
  }
  const agentID = typeof obj.agent_id === "string" ? obj.agent_id : "";
  const apiKey = typeof obj.api_key === "string" ? obj.api_key : "";
  const agentName = typeof obj.name === "string" && obj.name.trim() ? obj.name : fallbackName;
  if (!agentID || !apiKey || !agentName) {
    return null;
  }
  return {
    api_base: apiBase,
    agent_name: agentName,
    agent_id: agentID,
    api_key: apiKey
  };
}
