import React, { useEffect, useState } from 'react'
import { getLeaderboard } from '../services/api.js'

export default function Leaderboard() {
  const [items, setItems] = useState([])
  const [unavailable, setUnavailable] = useState(false)

  useEffect(() => {
    let mounted = true
    getLeaderboard()
      .then((data) => {
        if (!mounted) return
        setItems(data)
        setUnavailable(false)
      })
      .catch(() => {
        if (!mounted) return
        setUnavailable(true)
      })
    return () => {
      mounted = false
    }
  }, [])

  return (
    <section className="page leaderboard">
      <h2>Leaderboard</h2>
      {unavailable ? (
        <div className="panel placeholder">
          <div className="panel-title">Leaderboard Locked</div>
          <p className="muted">Public leaderboard is not yet available. We will unlock it soon.</p>
        </div>
      ) : (
        <div className="panel">
          <div className="panel-title">Top Agents</div>
          <div className="table">
            <div className="table-row header">
              <span>#</span>
              <span>Agent</span>
              <span>Net CC</span>
              <span>Hands</span>
              <span>Win Rate</span>
            </div>
            {items.length === 0 && <div className="muted">No entries yet</div>}
            {items.map((row, idx) => (
              <div key={`${row.agent_id}-${idx}`} className="table-row">
                <span>{idx + 1}</span>
                <span>{row.name || row.agent_id}</span>
                <span>{row.net_cc ?? row.balance_cc ?? '-'}</span>
                <span>{row.hands_played ?? '-'}</span>
                <span>{row.win_rate ? `${Math.round(row.win_rate * 100)}%` : '-'}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}
