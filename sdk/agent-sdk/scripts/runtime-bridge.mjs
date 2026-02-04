#!/usr/bin/env node
import readline from 'node:readline'

function parseArgs(argv) {
  const args = {}
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i]
    if (!token.startsWith('--')) continue
    const key = token.slice(2)
    const value = argv[i + 1]
    if (!value || value.startsWith('--')) {
      args[key] = true
      continue
    }
    args[key] = value
    i += 1
  }
  return args
}

function must(name, value) {
  if (!value || String(value).trim() === '') {
    throw new Error(`missing ${name}`)
  }
  return String(value).trim()
}

function emit(obj) {
  process.stdout.write(`${JSON.stringify(obj)}\n`)
}

async function parseSSE(url, lastEventID, onEvent) {
  const headers = { Accept: 'text/event-stream' }
  if (lastEventID) headers['Last-Event-ID'] = lastEventID
  const res = await fetch(url, { headers })
  if (!res.ok || !res.body) {
    throw new Error(`stream_open_failed_${res.status}`)
  }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let eventID = ''
  let eventName = ''
  let data = ''
  let latest = lastEventID

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    for (const rawLine of lines) {
      const line = rawLine.trimEnd()
      if (line.startsWith('id:')) {
        eventID = line.slice(3).trim()
        continue
      }
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim()
        continue
      }
      if (line.startsWith('data:')) {
        const chunk = line.slice(5).trimStart()
        data = data ? `${data}\n${chunk}` : chunk
        continue
      }
      if (line !== '') continue
      if (!data) {
        eventID = ''
        eventName = ''
        continue
      }
      if (eventID) latest = eventID
      await onEvent({ id: eventID, event: eventName, data })
      eventID = ''
      eventName = ''
      data = ''
    }
  }
  return latest
}

async function main() {
  const args = parseArgs(process.argv.slice(2))
  const apiBase = must('--api-base', args['api-base'] || process.env.API_BASE || 'http://localhost:8080/api')
  const agentID = must('--agent-id', args['agent-id'] || process.env.AGENT_ID)
  const apiKey = must('--api-key', args['api-key'] || process.env.APA_API_KEY)
  const joinMode = (args.join || 'random') === 'select' ? 'select' : 'random'
  const roomID = joinMode === 'select' ? must('--room-id', args['room-id']) : undefined

  const createRes = await fetch(`${apiBase}/agent/sessions`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      agent_id: agentID,
      api_key: apiKey,
      join_mode: joinMode,
      room_id: roomID
    })
  })
  if (!createRes.ok) {
    throw new Error(`create_session_failed_${createRes.status}:${await createRes.text()}`)
  }
  const created = await createRes.json()
  const sessionID = created.session_id
  const streamURL = created.stream_url.startsWith('http')
    ? created.stream_url
    : `${apiBase.replace(/\/api\/?$/, '')}${created.stream_url}`

  emit({ type: 'ready', session_id: sessionID, stream_url: streamURL })

  const pending = new Map()
  const seenTurns = new Set()
  let stopRequested = false
  let lastEventID = ''
  const rl = readline.createInterface({ input: process.stdin })

  rl.on('line', async (line) => {
    const raw = line.trim()
    if (!raw) return
    let msg
    try {
      msg = JSON.parse(raw)
    } catch {
      return
    }
    if (msg.type === 'stop') {
      stopRequested = true
      return
    }
    if (msg.type !== 'decision_response') return
    const reqID = String(msg.request_id || '')
    const pendingTurn = pending.get(reqID)
    if (!pendingTurn) return
    pending.delete(reqID)
    const actionRes = await fetch(`${apiBase}/agent/sessions/${sessionID}/actions`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        request_id: reqID,
        turn_id: pendingTurn.turnID,
        action: msg.action,
        amount: msg.amount,
        thought_log: msg.thought_log || ''
      })
    })
    let body = {}
    try {
      body = await actionRes.json()
    } catch {
      body = {}
    }
    emit({
      type: 'action_result',
      request_id: reqID,
      ok: actionRes.ok && body.accepted === true,
      body
    })
  })

  while (!stopRequested) {
    try {
      lastEventID = await parseSSE(streamURL, lastEventID, async (evt) => {
        let envelope
        try {
          envelope = JSON.parse(evt.data)
        } catch {
          return
        }
        const eventType = envelope?.event || evt.event
        const data = envelope?.data || {}
        emit({ type: 'server_event', event: eventType, event_id: evt.id || '' })
        if (eventType !== 'state_snapshot') return
        const turnID = String(data.turn_id || '')
        const mySeat = Number(data.my_seat ?? -1)
        const actorSeat = Number(data.current_actor_seat ?? -2)
        if (!turnID || mySeat !== actorSeat || seenTurns.has(turnID)) return
        seenTurns.add(turnID)
        const reqID = `req_${Date.now()}_${Math.floor(Math.random() * 1_000_000_000)}`
        pending.set(reqID, { turnID })
        emit({
          type: 'decision_request',
          request_id: reqID,
          session_id: sessionID,
          turn_id: turnID,
          legal_actions: ['fold', 'check', 'call', 'raise', 'bet'],
          state: data
        })
      })
    } catch (err) {
      emit({ type: 'stream_error', error: err instanceof Error ? err.message : String(err) })
      if (stopRequested) break
      await new Promise((resolve) => setTimeout(resolve, 500))
    }
  }

  rl.close()
  await fetch(`${apiBase}/agent/sessions/${sessionID}`, { method: 'DELETE' }).catch(() => undefined)
  emit({ type: 'stopped', session_id: sessionID })
}

main().catch((err) => {
  emit({ type: 'runtime_error', error: err instanceof Error ? err.message : String(err) })
  process.exit(1)
})
