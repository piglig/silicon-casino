import test from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import { promises as fs } from "node:fs";

import { runCLI } from "./cli.js";
import { saveCredential } from "./loop/credentials.js";
import { loadDecisionState } from "./loop/decision_state.js";

type FetchCall = {
  url: string;
  method: string;
  headers?: HeadersInit;
  body?: string;
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" }
  });
}

function sseResponse(chunks: string[]): Response {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    }
  });
  return new Response(stream, {
    status: 200,
    headers: { "content-type": "text/event-stream" }
  });
}

function captureStdout() {
  const writes: string[] = [];
  const original = process.stdout.write.bind(process.stdout);
  process.stdout.write = ((chunk: string | Uint8Array) => {
    writes.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf8"));
    return true;
  }) as typeof process.stdout.write;
  return {
    writes,
    restore() {
      process.stdout.write = original;
    }
  };
}

async function withTempCwd<T>(fn: () => Promise<T>): Promise<T> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "apa-sdk-next-decision-"));
  const prev = process.cwd();
  process.chdir(dir);
  try {
    return await fn();
  } finally {
    process.chdir(prev);
  }
}

test("next-decision creates session, emits decision_request, and updates state", async () => {
  await withTempCwd(async () => {
    const apiBase = "http://mock.local/api";
    await saveCredential({
      api_base: apiBase,
      agent_name: "BotA",
      agent_id: "agent_1",
      api_key: "apa_1"
    });

    const calls: FetchCall[] = [];
    const originalFetch = globalThis.fetch;
    const stdout = captureStdout();

    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method || "GET";
      calls.push({
        url,
        method,
        headers: init?.headers,
        body: typeof init?.body === "string" ? init.body : undefined
      });

      if (url === `${apiBase}/agents/me` && method === "GET") {
        return jsonResponse({ status: "claimed", balance_cc: 10000 });
      }
      if (url === `${apiBase}/public/rooms` && method === "GET") {
        return jsonResponse({ items: [{ id: "room_low", min_buyin_cc: 1000, name: "Low" }] });
      }
      if (url === `${apiBase}/agent/sessions` && method === "POST") {
        return jsonResponse({
          session_id: "sess_1",
          stream_url: "/api/agent/sessions/sess_1/events"
        });
      }
      if (url === `${apiBase}/agent/sessions/sess_1/events` && method === "GET") {
        return sseResponse([
          "id: 101\nevent: message\ndata: {\"event\":\"state_snapshot\",\"data\":{\"turn_id\":\"turn_1\",",
          "\"my_seat\":0,\"current_actor_seat\":0}}\n\n"
        ]);
      }
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }) as typeof globalThis.fetch;

    try {
      await runCLI(["next-decision", "--api-base", apiBase, "--join", "random", "--timeout-ms", "2000"]);
    } finally {
      globalThis.fetch = originalFetch;
      stdout.restore();
    }

    const messages = stdout.writes
      .join("")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => JSON.parse(line) as Record<string, unknown>);
    const decision = messages.find((m) => m.type === "decision_request");
    assert.ok(decision);
    assert.equal(decision?.session_id, "sess_1");
    assert.equal(decision?.turn_id, "turn_1");
    assert.equal(decision?.callback_url, `${apiBase}/agent/sessions/sess_1/actions`);

    const state = await loadDecisionState();
    assert.equal(state.session_id, "sess_1");
    assert.equal(state.stream_url, `${apiBase}/agent/sessions/sess_1/events`);
    assert.equal(state.last_event_id, "101");

    const created = calls.find((c) => c.url === `${apiBase}/agent/sessions` && c.method === "POST");
    assert.ok(created);
  });
});

test("next-decision recovers from 409 and reuses existing session from response body", async () => {
  await withTempCwd(async () => {
    const apiBase = "http://mock.local/api";
    await saveCredential({
      api_base: apiBase,
      agent_name: "BotA",
      agent_id: "agent_1",
      api_key: "apa_1"
    });

    const originalFetch = globalThis.fetch;
    const stdout = captureStdout();

    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method || "GET";

      if (url === `${apiBase}/agents/me` && method === "GET") {
        return jsonResponse({ status: "claimed", balance_cc: 10000 });
      }
      if (url === `${apiBase}/public/rooms` && method === "GET") {
        return jsonResponse({ items: [{ id: "room_low", min_buyin_cc: 1000, name: "Low" }] });
      }
      if (url === `${apiBase}/agent/sessions` && method === "POST") {
        return jsonResponse(
          {
            error: "agent_already_in_session",
            session_id: "sess_conflict",
            stream_url: "/api/agent/sessions/sess_conflict/events"
          },
          409
        );
      }
      if (url === `${apiBase}/agent/sessions/sess_conflict/events` && method === "GET") {
        return sseResponse([
          "id: 202\nevent: message\ndata: {\"event\":\"state_snapshot\",\"data\":{\"turn_id\":\"turn_2\",\"my_seat\":1,\"current_actor_seat\":1}}\n\n"
        ]);
      }
      throw new Error(`unexpected fetch: ${method} ${url}`);
    }) as typeof globalThis.fetch;

    try {
      await runCLI(["next-decision", "--api-base", apiBase, "--join", "random", "--timeout-ms", "2000"]);
    } finally {
      globalThis.fetch = originalFetch;
      stdout.restore();
    }

    const messages = stdout.writes
      .join("")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => JSON.parse(line) as Record<string, unknown>);
    const decision = messages.find((m) => m.type === "decision_request");
    assert.ok(decision);
    assert.equal(decision?.session_id, "sess_conflict");
    assert.equal(decision?.callback_url, `${apiBase}/agent/sessions/sess_conflict/actions`);

    const state = await loadDecisionState();
    assert.equal(state.session_id, "sess_conflict");
    assert.equal(state.stream_url, `${apiBase}/agent/sessions/sess_conflict/events`);
    assert.equal(state.last_event_id, "202");
  });
});
