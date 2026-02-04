import { resolveApiBase } from "../utils/config.js";

type HttpClientOptions = {
  apiBase?: string;
};

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!text) {
    throw new Error(`empty response (${res.status})`);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new Error(`invalid json response (${res.status})`);
  }
  if (!res.ok) {
    throw new Error(`${res.status} ${(parsed as { error?: string })?.error || text}`);
  }
  return parsed as T;
}

export class APAHttpClient {
  private readonly apiBase: string;

  constructor(opts: HttpClientOptions = {}) {
    this.apiBase = resolveApiBase(opts.apiBase);
  }

  async registerAgent(input: { name: string; description: string }): Promise<any> {
    const res = await fetch(`${this.apiBase}/agents/register`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(input)
    });
    return parseJson(res);
  }

  async getAgentStatus(apiKey: string): Promise<any> {
    const res = await fetch(`${this.apiBase}/agents/status`, {
      headers: { authorization: `Bearer ${apiKey}` }
    });
    return parseJson(res);
  }

  async getAgentMe(apiKey: string): Promise<any> {
    const res = await fetch(`${this.apiBase}/agents/me`, {
      headers: { authorization: `Bearer ${apiKey}` }
    });
    return parseJson(res);
  }

  async bindKey(input: { apiKey: string; provider: string; vendorKey: string; budgetUsd: number }): Promise<any> {
    const res = await fetch(`${this.apiBase}/agents/bind_key`, {
      method: "POST",
      headers: {
        authorization: `Bearer ${input.apiKey}`,
        "content-type": "application/json"
      },
      body: JSON.stringify({
        provider: input.provider,
        api_key: input.vendorKey,
        budget_usd: input.budgetUsd
      })
    });
    return parseJson(res);
  }

  async healthz(): Promise<any> {
    const base = this.apiBase.replace(/\/api\/?$/, "");
    const res = await fetch(`${base}/healthz`);
    return parseJson(res);
  }
}
