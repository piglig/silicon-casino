import React, { useEffect } from 'react'
import { useParams } from 'react-router-dom'
import TableScene from '../pixi/TableScene.jsx'
import TableHUD from '../components/TableHUD.jsx'
import ActionLog from '../components/ActionLog.jsx'
import ThoughtLog from '../components/ThoughtLog.jsx'
import { useSpectatorStore } from '../state/useSpectatorStore.jsx'

export default function Match() {
  const { roomId } = useParams()
  const { snapshot, lastEvent, eventLogs, thoughtLogs, showdown, status, connect, timeLeftMs } =
    useSpectatorStore()

  useEffect(() => {
    if (roomId) connect(roomId)
  }, [roomId])

  return (
    <section className="page match">
      <div className="match-header">
        <div className="match-title">
          <span>Room</span> <strong>{roomId}</strong>
        </div>
        <div className={`status-pill ${status}`}>{status}</div>
      </div>
      <div className="match-grid">
        <div className="table-wrap">
          <TableScene snapshot={snapshot} eventLog={lastEvent} showdown={showdown} />
          <ThoughtLog items={thoughtLogs} />
        </div>
        <div className="match-side">
          <TableHUD snapshot={snapshot} timeLeftMs={timeLeftMs} />
          <ActionLog items={eventLogs} />
          <div className="panel showdown">
            <div className="panel-title">Showdown</div>
            {showdown.length === 0 && <div className="muted">No showdown yet</div>}
            {showdown.map((s, i) => (
              <div key={`${s.agent_id}-${i}`} className="showdown-line">
                <span>{s.agent_id}</span>
                <span className="cards">{(s.hole_cards || []).join(' ') || '-'}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
