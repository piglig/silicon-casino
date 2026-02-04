#!/usr/bin/env node
import http from 'node:http'
import { spawn } from 'node:child_process'
import readline from 'node:readline'

const PORT = 19081
const API_BASE = `http://127.0.0.1:${PORT}/api`

let nextEventID = 1
let sseClient = null
let receivedAction = null

function writeSSE(res, event, data) {
  const id = String(nextEventID++)
  const envelope = {
    event_id: id,
    event,
    session_id: 'sess_demo',
    server_ts: Date.now(),
    data
  }
  res.write(`id: ${id}\n`)
  res.write(`event: ${event}\n`)
  res.write(`data: ${JSON.stringify(envelope)}\n\n`)
}

function readJSONBody(req) {
  return new Promise((resolve, reject) => {
    let buf = ''
    req.on('data', (c) => { buf += c.toString('utf8') })
    req.on('end', () => {
      try {
        resolve(buf ? JSON.parse(buf) : {})
      } catch (err) {
        reject(err)
      }
    })
    req.on('error', reject)
  })
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'POST' && req.url === '/api/agent/sessions') {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({
      session_id: 'sess_demo',
      stream_url: `/api/agent/sessions/sess_demo/events`,
      room_id: 'room_demo'
    }))
    return
  }

  if (req.method === 'GET' && req.url === '/api/agent/sessions/sess_demo/events') {
    res.writeHead(200, {
      'content-type': 'text/event-stream',
      'cache-control': 'no-cache',
      connection: 'keep-alive'
    })
    sseClient = res
    writeSSE(res, 'session_joined', { room_id: 'room_demo', seat_id: 0, table_id: 'tbl_demo' })
    setTimeout(() => {
      if (!sseClient) return
      writeSSE(res, 'state_snapshot', {
        hand_id: 'hand_1',
        turn_id: 'turn_1',
        my_seat: 0,
        current_actor_seat: 0,
        pot: 200,
        action_timeout_ms: 5000
      })
    }, 200)
    req.on('close', () => {
      if (sseClient === res) sseClient = null
    })
    return
  }

  if (req.method === 'POST' && req.url === '/api/agent/sessions/sess_demo/actions') {
    receivedAction = await readJSONBody(req)
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ accepted: true, request_id: receivedAction.request_id }))
    if (sseClient) {
      writeSSE(sseClient, 'hand_end', { winner: 'agent_demo', pot: 200, balances: [] })
    }
    return
  }

  if (req.method === 'DELETE' && req.url === '/api/agent/sessions/sess_demo') {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({ ok: true }))
    return
  }

  res.statusCode = 404
  res.end('not found')
})

async function main() {
  await new Promise((resolve) => server.listen(PORT, '127.0.0.1', resolve))
  console.log(`[demo] mock game-server listening on ${API_BASE}`)

  const runtime = spawn('node', [
    'sdk/agent-sdk/scripts/runtime-bridge.mjs',
    '--api-base', API_BASE,
    '--agent-id', 'agent_demo',
    '--api-key', 'apa_demo',
    '--join', 'random'
  ], { stdio: ['pipe', 'pipe', 'pipe'] })

  const rl = readline.createInterface({ input: runtime.stdout })
  let decisionCount = 0
  let actionResultOK = false

  rl.on('line', (line) => {
    let msg
    try {
      msg = JSON.parse(line)
    } catch {
      return
    }
    console.log(`[runtime->agent] ${line}`)
    if (msg.type === 'decision_request') {
      decisionCount += 1
      runtime.stdin.write(`${JSON.stringify({
        type: 'decision_response',
        request_id: msg.request_id,
        action: 'call',
        thought_log: 'demo decision'
      })}\n`)
    }
    if (msg.type === 'action_result' && msg.ok === true) {
      actionResultOK = true
      runtime.stdin.write('{"type":"stop"}\n')
    }
  })

  const exitCode = await new Promise((resolve) => {
    runtime.on('exit', (code) => resolve(code ?? 1))
  })

  server.close()

  console.log(`[demo] runtime exited with code=${exitCode}`)
  console.log(`[demo] decision requests received=${decisionCount}`)
  console.log(`[demo] action posted=${JSON.stringify(receivedAction)}`)

  if (exitCode !== 0 || decisionCount < 1 || !actionResultOK || !receivedAction) {
    process.exit(1)
  }
  console.log('[demo] PASS: CLI agent received decision request and returned action successfully')
}

main().catch((err) => {
  console.error(err)
  server.close()
  process.exit(1)
})
