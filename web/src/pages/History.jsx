import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getTableHistory } from '../services/api.js'

export default function History() {
  const [items, setItems] = useState([])
  const [roomId, setRoomId] = useState('')
  const [agentId, setAgentId] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = () => {
    setLoading(true)
    setError('')
    getTableHistory({ roomId: roomId.trim(), agentId: agentId.trim(), limit: 100, offset: 0 })
      .then((res) => setItems(res.items || []))
      .catch((err) => setError(err?.message || 'history_load_failed'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  return (
    <section className="page">
      <div className="panel">
        <div className="panel-title">Table History</div>
        <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
          <input placeholder="Room ID" value={roomId} onChange={(e) => setRoomId(e.target.value)} />
          <input placeholder="Agent ID" value={agentId} onChange={(e) => setAgentId(e.target.value)} />
          <button className="btn btn-primary" onClick={load} disabled={loading}>
            {loading ? 'Loading...' : 'Search'}
          </button>
        </div>
        {error && <div className="muted" style={{ marginTop: 8 }}>{error}</div>}
      </div>

      <div className="panel" style={{ marginTop: 12 }}>
        <div className="panel-title">Results</div>
        {items.length === 0 && !loading && <div className="muted">No tables found</div>}
        {items.map((it) => (
          <div key={it.table_id} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: '8px 0', borderBottom: '1px solid #2a2a2a' }}>
            <div>
              <div><strong>{it.table_id}</strong></div>
              <div className="muted">room={it.room_id} status={it.status} blinds={it.small_blind_cc}/{it.big_blind_cc}</div>
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              <Link className="btn btn-ghost" to={`/replay/${it.table_id}`}>Replay</Link>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
