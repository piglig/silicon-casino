import React, { useMemo, useRef, useState, useEffect } from 'react'
import PixiTable from './PixiTable.jsx'

const defaultUrl = (window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host + '/ws'

export default function App() {
  const [url, setUrl] = useState(defaultUrl)
  const [status, setStatus] = useState('disconnected')
  const [snapshot, setSnapshot] = useState(null)
  const [logs, setLogs] = useState([])
  const [lastEvent, setLastEvent] = useState(null)
  const [showdown, setShowdown] = useState([])
  const [rooms, setRooms] = useState([])
  const [agentId, setAgentId] = useState('bot')
  const [apiKey, setApiKey] = useState('')
  const [roomId, setRoomId] = useState('')
  const [regName, setRegName] = useState('')
  const [regDesc, setRegDesc] = useState('')
  const [regResult, setRegResult] = useState(null)
  const [claimAgentId, setClaimAgentId] = useState('')
  const [claimCode, setClaimCode] = useState('')
  const [claimResult, setClaimResult] = useState(null)
  const [statusKey, setStatusKey] = useState('')
  const [statusResult, setStatusResult] = useState(null)
  const [adminKey, setAdminKey] = useState(localStorage.getItem('apa_admin_key') || '')
  const [errorMsg, setErrorMsg] = useState('')
  const wsRef = useRef(null)

  const appendLog = (line) => {
    setLogs((prev) => {
      const next = [...prev, `[${new Date().toLocaleTimeString()}] ${line}`]
      return next.slice(-200)
    })
  }

  useEffect(() => {
    if (adminKey) {
      localStorage.setItem('apa_admin_key', adminKey)
    } else {
      localStorage.removeItem('apa_admin_key')
    }
  }, [adminKey])

  const connect = (mode) => {
    if (wsRef.current) wsRef.current.close()
    const ws = new WebSocket(url)
    wsRef.current = ws
    setStatus('connecting')
    ws.onopen = () => {
      setStatus('connected')
      if (mode === 'spectate') {
        ws.send(JSON.stringify({ type: 'spectate', room_id: roomId || undefined }))
      } else if (mode === 'random') {
        ws.send(JSON.stringify({ type: 'join', agent_id: agentId, api_key: apiKey, join_mode: 'random' }))
      } else if (mode === 'select') {
        ws.send(JSON.stringify({ type: 'join', agent_id: agentId, api_key: apiKey, join_mode: 'select', room_id: roomId }))
      }
    }
    ws.onclose = () => setStatus('disconnected')
    ws.onerror = () => setStatus('error')
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'state_update') {
          setSnapshot(msg)
        } else if (msg.type === 'event_log') {
          const tl = msg.thought_log ? ` thought="${msg.thought_log}"` : ''
          appendLog(`event=${msg.event} seat=${msg.player_seat} action=${msg.action} amount=${msg.amount || 0}${tl}`)
          setLastEvent(msg)
        } else if (msg.type === 'join_result') {
          appendLog(`join_result ok=${msg.ok} room=${msg.room_id || ''} err=${msg.error || ''}`)
        } else if (msg.type === 'hand_end') {
          appendLog(`hand_end winner=${msg.winner} pot=${msg.pot}`)
          setShowdown(msg.showdown || [])
        } else if (msg.type === 'action_result') {
          appendLog(`action_result ok=${msg.ok} err=${msg.error || ''}`)
        }
      } catch (e) {
        appendLog('parse_error')
      }
    }
  }

  const loadRooms = async () => {
    try {
      const headers = adminKey ? { 'X-Admin-Key': adminKey } : undefined
      const res = await fetch('/api/rooms', { headers })
      if (!res.ok) {
        appendLog(`rooms_fetch_failed status=${res.status}`)
        return
      }
      const data = await res.json()
      setRooms(data.items || [])
      if (!roomId && data.items?.length) setRoomId(data.items[0].id)
    } catch (e) {
      appendLog('rooms_fetch_failed')
    }
  }

  const registerAgent = async () => {
    setErrorMsg('')
    setRegResult(null)
    try {
      const res = await fetch('/api/agents/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: regName, description: regDesc })
      })
      if (!res.ok) {
        setErrorMsg('register_failed')
        return
      }
      const data = await res.json()
      setRegResult(data.agent || null)
    } catch (e) {
      setErrorMsg('register_failed')
    }
  }

  const claimAgent = async () => {
    setErrorMsg('')
    setClaimResult(null)
    try {
      const res = await fetch('/api/agents/claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent_id: claimAgentId, claim_code: claimCode })
      })
      if (!res.ok) {
        setErrorMsg('invalid_claim')
        return
      }
      const data = await res.json()
      setClaimResult(data)
    } catch (e) {
      setErrorMsg('invalid_claim')
    }
  }

  const checkStatus = async () => {
    setErrorMsg('')
    setStatusResult(null)
    try {
      const res = await fetch('/api/agents/status', {
        headers: { Authorization: `Bearer ${statusKey}` }
      })
      if (!res.ok) {
        setErrorMsg('invalid_api_key')
        return
      }
      const data = await res.json()
      setStatusResult(data)
    } catch (e) {
      setErrorMsg('invalid_api_key')
    }
  }

  useEffect(() => {
    loadRooms()
    const id = setInterval(() => loadRooms(), 5000)
    return () => clearInterval(id)
  }, [adminKey])

  const community = useMemo(() => (snapshot?.community_cards || []).join(' '), [snapshot])
  const opp = snapshot?.opponents?.[0]

  return (
    <div className="page">
      <header>
        <div className="title">APA Debug UI</div>
        <div className="connection">
          <input value={url} onChange={(e) => setUrl(e.target.value)} />
          <input
            placeholder="Admin Key"
            value={adminKey}
            onChange={(e) => setAdminKey(e.target.value)}
          />
          <button onClick={() => connect('spectate')}>Spectate</button>
          <button onClick={() => connect('random')}>Join Random</button>
          <button onClick={() => connect('select')}>Join Room</button>
          <button onClick={loadRooms}>Load Rooms</button>
          <span className={`status ${status}`}>{status}</span>
        </div>
      </header>

      <section className="grid">
        <div className="card pixi">
          <h3>Table View</h3>
          <PixiTable snapshot={snapshot} eventLog={lastEvent} />
        </div>

        <div className="card">
          <h3>Table</h3>
          <div>Game: {snapshot?.game_id || '-'}</div>
          <div>Hand: {snapshot?.hand_id || '-'}</div>
          <div>Street: {snapshot?.street || '-'}</div>
          <div>Pot: {snapshot?.pot ?? '-'}</div>
          <div>Community: {community || '-'}</div>
          <div>Min Raise: {snapshot?.min_raise ?? '-'}</div>
          <div>Current Bet: {snapshot?.current_bet ?? '-'}</div>
          <div>Call Amount: {snapshot?.call_amount ?? '-'}</div>
          <div>Actor Seat: {snapshot?.current_actor_seat ?? '-'}</div>
        </div>

        <div className="card">
          <h3>Players / Join</h3>
          <div className="field">
            <label>Agent ID</label>
            <input value={agentId} onChange={(e) => setAgentId(e.target.value)} />
          </div>
          <div className="field">
            <label>API Key</label>
            <input value={apiKey} onChange={(e) => setApiKey(e.target.value)} />
          </div>
          <div className="field">
            <label>Room</label>
            <select value={roomId} onChange={(e) => setRoomId(e.target.value)}>
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name} (min {r.min_buyin_cc})
                </option>
              ))}
            </select>
          </div>
          <div className="player">
            <div>Me</div>
            <div>Balance: {snapshot?.my_balance ?? '-'}</div>
          </div>
          <div className="player">
            <div>Opponent: {opp?.name || '-'}</div>
            <div>Seat: {opp?.seat ?? '-'}</div>
            <div>Stack: {opp?.stack ?? '-'}</div>
            <div>Last Action: {opp?.action ?? '-'}</div>
          </div>
        </div>

        <div className="card">
          <h3>Agent Claim</h3>
          {errorMsg && <div className="error">{errorMsg}</div>}
          <div className="field">
            <label>Register Name</label>
            <input value={regName} onChange={(e) => setRegName(e.target.value)} />
          </div>
          <div className="field">
            <label>Register Description</label>
            <input value={regDesc} onChange={(e) => setRegDesc(e.target.value)} />
          </div>
          <button onClick={registerAgent}>Register</button>
          {regResult && (
            <div className="result">
              <div>agent_id: <span className="mono">{regResult.agent_id}</span></div>
              <div>api_key: <span className="mono">{regResult.api_key}</span> (show once)</div>
              <div>claim_code: <span className="mono">{regResult.verification_code}</span></div>
              <div>claim_url: <span className="mono">{regResult.claim_url}</span></div>
            </div>
          )}

          <div className="field">
            <label>Claim Agent ID</label>
            <input value={claimAgentId} onChange={(e) => setClaimAgentId(e.target.value)} />
          </div>
          <div className="field">
            <label>Claim Code</label>
            <input value={claimCode} onChange={(e) => setClaimCode(e.target.value)} />
          </div>
          <button onClick={claimAgent}>Claim</button>
          {claimResult && <div className="result">claim ok</div>}

          <div className="field">
            <label>Status API Key</label>
            <input value={statusKey} onChange={(e) => setStatusKey(e.target.value)} />
          </div>
          <button onClick={checkStatus}>Check Status</button>
          {statusResult && <div className="result">status: {statusResult.status}</div>}
        </div>

        <div className="card logs">
          <h3>Logs</h3>
          <div className="log-list">
            {logs.map((l, i) => (
              <div key={i} className="log-line">{l}</div>
            ))}
          </div>
        </div>

        <div className="card">
          <h3>Showdown</h3>
          {showdown.length === 0 && <div>-</div>}
          {showdown.map((s, i) => (
            <div key={i} className="showdown-line">
              <span className="mono">{s.agent_id}</span>
              <span className="cards">{(s.hole_cards || []).join(' ') || '-'}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}
