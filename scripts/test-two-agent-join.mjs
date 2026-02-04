#!/usr/bin/env node

const args = parseArgs(process.argv.slice(2));
const rawBase = args["api-base"] || process.env.API_BASE || "http://localhost:8080";
const base = normalizeApiBase(rawBase);

async function main() {
  const bot1 = await registerAgent("join-test-a");
  const bot2 = await registerAgent("join-test-b");

  const session1 = await createSession(bot1, "random");
  const session2 = await createSession(bot2, "random");

  try {
    const logs1 = await collectNonPingEvents(session1.session_id, 4);
    const logs2 = await collectNonPingEvents(session2.session_id, 3);

    console.log(`[session ${session1.session_id}] ${logs1.map((e) => e.event).join(" -> ")}`);
    console.log(`[session ${session2.session_id}] ${logs2.map((e) => e.event).join(" -> ")}`);

    assertContainsInOrder(
      logs1.map((e) => e.event),
      ["session_joined", "session_joined", "state_snapshot", "turn_started"],
      "session1"
    );
    assertContainsInOrder(
      logs2.map((e) => e.event),
      ["session_joined", "state_snapshot", "turn_started"],
      "session2"
    );

    for (const ev of [...logs1, ...logs2]) {
      if (!ev.id) {
        throw new Error(`event id is empty for event=${ev.event}`);
      }
    }

    console.log("[PASS] two-agent join flow emitted expected SSE events");
  } finally {
    await Promise.allSettled([
      closeSession(session1.session_id),
      closeSession(session2.session_id)
    ]);
  }
}

async function registerAgent(name) {
  const res = await fetch(`${base}/agents/register`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name, description: "join flow test" })
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(`register failed: ${res.status} ${JSON.stringify(body)}`);
  }
  if (!body?.agent?.agent_id || !body?.agent?.api_key) {
    throw new Error(`register response missing credentials: ${JSON.stringify(body)}`);
  }
  return {
    agent_id: body.agent.agent_id,
    api_key: body.agent.api_key
  };
}

async function createSession(agent, joinMode) {
  const res = await fetch(`${base}/agent/sessions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      agent_id: agent.agent_id,
      api_key: agent.api_key,
      join_mode: joinMode
    })
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(`create session failed: ${res.status} ${JSON.stringify(body)}`);
  }
  if (!body?.session_id) {
    throw new Error(`create session missing session_id: ${JSON.stringify(body)}`);
  }
  return body;
}

async function closeSession(sessionId) {
  await fetch(`${base}/agent/sessions/${sessionId}`, { method: "DELETE" });
}

async function collectNonPingEvents(sessionId, want, timeoutMs = 8000) {
  const res = await fetch(`${base}/agent/sessions/${sessionId}/events`, {
    headers: { accept: "text/event-stream" }
  });
  if (!res.ok || !res.body) {
    throw new Error(`open sse failed: ${res.status}`);
  }

  const reader = res.body
    .pipeThrough(new TextDecoderStream())
    .getReader();

  const events = [];
  let buffer = "";
  const deadline = Date.now() + timeoutMs;

  while (events.length < want) {
    if (Date.now() > deadline) {
      throw new Error(`timeout waiting events for ${sessionId}: got ${events.length}/${want}`);
    }
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += value;
    const chunks = buffer.split("\n\n");
    buffer = chunks.pop() || "";

    for (const chunk of chunks) {
      const event = parseSSEChunk(chunk);
      if (!event || event.event === "ping") {
        continue;
      }
      events.push(event);
      if (events.length >= want) {
        await reader.cancel().catch(() => undefined);
        return events;
      }
    }
  }

  await reader.cancel().catch(() => undefined);
  return events;
}

function parseSSEChunk(chunk) {
  let id = "";
  let event = "message";

  for (const line of chunk.split("\n")) {
    if (line.startsWith("id: ")) {
      id = line.slice(4).trim();
    } else if (line.startsWith("event: ")) {
      event = line.slice(7).trim();
    }
  }

  return { id, event };
}

function assertContainsInOrder(got, expected, label) {
  let j = 0;
  for (const item of got) {
    if (item === expected[j]) {
      j += 1;
      if (j === expected.length) {
        return;
      }
    }
  }
  throw new Error(`${label} events mismatch: got=${JSON.stringify(got)} expectedInOrder=${JSON.stringify(expected)}`);
}

function normalizeApiBase(input) {
  let out = String(input || "").trim().replace(/\/+$/, "");
  if (!out.endsWith("/api")) {
    out = `${out}/api`;
  }
  return out;
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) {
      continue;
    }
    const key = token.slice(2);
    const next = argv[i + 1];
    if (!next || next.startsWith("--")) {
      out[key] = true;
      continue;
    }
    out[key] = next;
    i += 1;
  }
  return out;
}

main().catch((err) => {
  console.error(`[FAIL] ${err instanceof Error ? err.message : String(err)}`);
  process.exit(1);
});
