import React, { useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import ReplayActionTicker from '../components/ReplayActionTicker.jsx'
import ReplayTableStage from '../components/ReplayTableStage.jsx'
import ReplayThoughtTimeline from '../components/ReplayThoughtTimeline.jsx'
import { useSpectatorStore } from '../state/useSpectatorStore.jsx'

export default function Match() {
  const { roomId } = useParams()
  const { snapshot, lastEvent, eventLogs, thoughtLogs, showdown, status, connect, timeLeftMs } =
    useSpectatorStore()

  useEffect(() => {
    if (roomId) connect({ roomId })
  }, [roomId])

  const liveState = useMemo(() => {
    if (!snapshot) return null
    const seats = (snapshot.seats || []).map((s) => ({
      seat_id: s.seat_id,
      agent_id: s.agent_id || `seat-${s.seat_id}`,
      agent_name: s.agent_name || s.agent_id || `Seat ${s.seat_id}`,
      stack: s.stack,
      hole_cards: s.hole_cards || null
    }))
    return {
      table_id: '',
      hand_id: snapshot.hand_id || '',
      street: snapshot.street || '-',
      pot_cc: snapshot.pot ?? 0,
      board_cards: snapshot.community_cards || [],
      current_actor_seat: snapshot.current_actor_seat,
      seats
    }
  }, [snapshot])

  const currentEvent = useMemo(() => ({
    seat_id: lastEvent?.player_seat,
    thought_log: lastEvent?.thought_log || ''
  }), [lastEvent])

  const thoughtItems = useMemo(
    () => thoughtLogs.map((it, i) => ({ global_seq: i + 1, seat_id: it.seat, thought_log: it.text })).reverse(),
    [thoughtLogs]
  )

  const actionItems = useMemo(
    () =>
      eventLogs
        .map((it, i) => ({
          global_seq: i + 1,
          seat_id: it.seat,
          action: it.action,
          amount_cc: it.amount
        }))
        .reverse(),
    [eventLogs]
  )

  return (
    <section className="page replay-page">
      <div className="replay-shell-clean">
        <header className="replay-controls-clean">
          <div className="match-title">
            <span>Room</span> <strong>{roomId}</strong>
          </div>
          <span className="muted">action timeout: {timeLeftMs != null ? `${Math.ceil(timeLeftMs / 1000)}s` : '-'}</span>
          <div className={`status-pill ${status}`}>{status}</div>
        </header>

        <div className="replay-main-clean">
          <ReplayTableStage state={liveState} currentEvent={currentEvent} handResult={null} />
          <aside className="replay-side-clean">
            <ReplayThoughtTimeline items={thoughtItems} activeSeq={thoughtItems[0]?.global_seq} />
            <ReplayActionTicker items={actionItems} activeSeq={actionItems[0]?.global_seq} />
            <div className="replay-panel replay-actions">
              <div className="replay-panel-title">Showdown</div>
              <div className="replay-panel-list">
                {showdown.length === 0 && <div className="muted">No showdown yet</div>}
                {showdown.map((s, i) => (
                  <div key={`${s.agent_id}-${i}`} className="replay-log-line">
                    <span className="replay-log-seat">{s.agent_id}</span>
                    <span className="replay-log-text">{(s.hole_cards || []).join(' ') || '-'}</span>
                  </div>
                ))}
              </div>
            </div>
          </aside>
        </div>
      </div>
    </section>
  )
}
