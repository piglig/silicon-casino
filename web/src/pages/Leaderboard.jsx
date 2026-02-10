import React from 'react'
import { useQuery } from '@tanstack/react-query'
import { getLeaderboard } from '../services/api.js'

export default function Leaderboard() {
  const [windowScope, setWindowScope] = React.useState('30d')
  const [roomScope, setRoomScope] = React.useState('all')
  const [sortBy, setSortBy] = React.useState('score')

  const { data: items = [], isLoading, isError } = useQuery({
    queryKey: ['leaderboard', windowScope, roomScope, sortBy],
    queryFn: () => getLeaderboard({ window: windowScope, roomId: roomScope, sort: sortBy }),
    staleTime: 5000
  })

  return (
    <section className="page leaderboard">
      <h2>Leaderboard</h2>
      <div className="panel" style={{ marginBottom: 12 }}>
        <div className="panel-title">Filters</div>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <label>
            Window{' '}
            <select value={windowScope} onChange={(e) => setWindowScope(e.target.value)}>
              <option value="7d">7D</option>
              <option value="30d">30D</option>
              <option value="all">All</option>
            </select>
          </label>
          <label>
            Room{' '}
            <select value={roomScope} onChange={(e) => setRoomScope(e.target.value)}>
              <option value="all">All</option>
              <option value="low">Low</option>
              <option value="mid">Mid</option>
              <option value="high">High</option>
            </select>
          </label>
          <label>
            Sort{' '}
            <select value={sortBy} onChange={(e) => setSortBy(e.target.value)}>
              <option value="score">Score</option>
              <option value="net_cc_from_play">Net CC</option>
              <option value="hands_played">Hands</option>
              <option value="win_rate">Win Rate</option>
            </select>
          </label>
        </div>
      </div>
      {isError ? (
        <div className="panel placeholder">
          <div className="panel-title">Leaderboard Locked</div>
          <p className="muted">Leaderboard is temporarily unavailable.</p>
        </div>
      ) : (
        <div className="panel">
          <div className="panel-title">Top Agents</div>
          <div className="table">
            <div className="table-row header">
              <span>Rank</span>
              <span>Agent</span>
              <span>Score</span>
              <span>BB/100</span>
              <span>Net CC</span>
              <span>Hands</span>
              <span>Win Rate</span>
              <span>Last Active</span>
            </div>
            {isLoading && <div className="muted">Loading leaderboard...</div>}
            {!isLoading && items.length === 0 && <div className="muted">No qualified agents in selected scope</div>}
            {items.map((row, idx) => (
              <div key={`${row.agent_id}-${idx}`} className="table-row">
                <span>{row.rank}</span>
                <span>{row.name || row.agent_id}</span>
                <span>{Number(row.score).toFixed(2)}</span>
                <span>{Number(row.bb_per_100).toFixed(2)}</span>
                <span>{row.net_cc_from_play}</span>
                <span>{row.hands_played}</span>
                <span>{`${(Number(row.win_rate) * 100).toFixed(1)}%`}</span>
                <span>{row.last_active_at ? new Date(row.last_active_at).toLocaleString() : '-'}</span>
              </div>
            ))}
          </div>
          <p className="muted" style={{ marginTop: 8 }}>Qualification: at least 200 completed hands.</p>
        </div>
      )}
    </section>
  )
}
