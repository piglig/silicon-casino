import React from 'react'

export default function ReplayThoughtTimeline({ items, activeSeq }) {
  return (
    <div className="replay-panel replay-thoughts">
      <div className="replay-panel-title">Thought Timeline</div>
      <div className="replay-panel-list">
        {items.length === 0 && <div className="muted">No thought logs yet</div>}
        {items.map((it) => {
          const cls = it.global_seq === activeSeq ? 'replay-log-line is-active' : 'replay-log-line'
          return (
            <div className={cls} key={`${it.global_seq}-${it.seat_id}-${it.thought_log.slice(0, 20)}`}>
              <span className="replay-log-seq">#{it.global_seq}</span>
              <span className="replay-log-seat">S{it.seat_id}</span>
              <span className="replay-log-text">{it.thought_log}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
