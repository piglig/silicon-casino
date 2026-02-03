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
  const wsRef = useRef(null)

  const appendLog = (line) => {
    setLogs((prev) => {
      const next = [...prev, `[${new Date().toLocaleTimeString()}] ${line}`]
      return next.slice(-200)
    })
  }

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
      const res = await fetch('/api/rooms')
      const data = await res.json()
      setRooms(data.items || [])
      if (!roomId && data.items?.length) setRoomId(data.items[0].id)
    } catch (e) {
      appendLog('rooms_fetch_failed')
    }
  }

  useEffect(() => {
    loadRooms()
    const id = setInterval(loadRooms, 5000)
    return () => clearInterval(id)
  }, [])

  const community = useMemo(() => (snapshot?.community_cards || []).join(' '), [snapshot])
  const opp = snapshot?.opponents?.[0]

  return (
    <div className="page">
      <header>
        <div className="title">APA Debug UI</div>
        <div className="connection">
          <input value={url} onChange={(e) => setUrl(e.target.value)} />
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
