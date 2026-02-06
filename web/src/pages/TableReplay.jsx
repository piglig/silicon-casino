import React, { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import ReplayActionTicker from '../components/ReplayActionTicker.jsx'
import ReplayTableStage from '../components/ReplayTableStage.jsx'
import ReplayThoughtTimeline from '../components/ReplayThoughtTimeline.jsx'
import { getTableReplay, getTableSnapshot, getTableTimeline } from '../services/api.js'

function toSeatMap(payload) {
  const seatMeta = new Map()
  for (const it of payload?.seat_map || []) {
    seatMeta.set(it.seat_id, {
      agent_id: it.agent_id,
      agent_name: it.agent_name
    })
  }

  const seats = []
  for (const s of payload?.stacks || []) {
    const m = seatMeta.get(s.seat_id) || {}
    seats.push({
      seat_id: s.seat_id,
      agent_id: m.agent_id || s.agent_id || `seat-${s.seat_id}`,
      agent_name: m.agent_name || s.agent_id || `Seat ${s.seat_id}`,
      stack: s.stack,
      hole_cards: null,
      last_action: s.last_action,
      last_action_amount: s.last_action_amount
    })
  }
  return seats
}

function buildHoleCardsByHand(events) {
  const map = new Map()
  for (const ev of events) {
    if (ev.event_type !== 'showdown') continue
    const handID = ev.hand_id || ev.payload?.hand_id
    if (!handID) continue
    const handMap = map.get(handID) || new Map()
    for (const row of ev.payload?.showdown || []) {
      if (!row?.agent_id) continue
      handMap.set(row.agent_id, row.hole_cards || [])
    }
    map.set(handID, handMap)
  }
  return map
}

function normalizeEvent(raw) {
  return {
    id: raw.id,
    hand_id: raw.hand_id,
    global_seq: raw.global_seq,
    event_type: raw.event_type,
    actor_agent_id: raw.actor_agent_id,
    payload: raw.payload || {}
  }
}

function stateAt(events, index, overrideState, holeByHand) {
  if (overrideState) return overrideState
  let out = null
  for (let i = 0; i <= index; i += 1) {
    const ev = events[i]
    if (ev?.event_type !== 'state_snapshot') continue
    const payload = ev.payload || {}
    const seats = toSeatMap(payload)
    const handID = payload.hand_id || ev.hand_id
    const holes = holeByHand.get(handID) || new Map()
    for (const s of seats) {
      s.hole_cards = holes.get(s.agent_id) || null
    }
    out = {
      table_id: payload.table_id,
      hand_id: handID,
      global_seq: ev.global_seq,
      street: payload.street,
      pot_cc: payload.pot_cc ?? payload.pot ?? 0,
      board_cards: payload.board_cards || payload.community_cards || [],
      current_actor_seat: payload.current_actor_seat,
      seats
    }
  }
  return out
}

function buildThoughtItems(events) {
  const out = []
  for (const ev of events) {
    const t = ev.payload?.thought_log
    if (!t) continue
    out.push({ global_seq: ev.global_seq, seat_id: ev.payload?.seat_id ?? '?', thought_log: t })
  }
  return out
}

function buildActionItems(events) {
  const out = []
  for (const ev of events) {
    if (ev.event_type !== 'action_applied') continue
    out.push({
      global_seq: ev.global_seq,
      seat_id: ev.payload?.seat_id,
      action: ev.payload?.action,
      amount_cc: ev.payload?.amount_cc,
      event_type: ev.event_type
    })
  }
  return out
}

function handResultAt(events, index) {
  let last = null
  for (let i = 0; i <= index; i += 1) {
    const ev = events[i]
    if (!ev) continue
    if (ev.event_type === 'hand_settled') {
      last = {
        hand_id: ev.payload?.hand_id || ev.hand_id || '',
        winner_agent_id: ev.payload?.winner || ev.actor_agent_id || '',
        pot_cc: ev.payload?.pot_cc ?? 0,
        global_seq: ev.global_seq
      }
      continue
    }
    if (ev.event_type === 'hand_started' && last && ev.global_seq > last.global_seq) {
      last = null
    }
  }
  return last
}

export default function TableReplay() {
  const { tableId } = useParams()
  const [events, setEvents] = useState([])
  const [timeline, setTimeline] = useState([])
  const [index, setIndex] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [speed, setSpeed] = useState(1)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [stateOverride, setStateOverride] = useState(null)

  useEffect(() => {
    let dead = false
    const load = async () => {
      setLoading(true)
      setError('')
      try {
        const all = []
        let fromSeq = 1
        for (;;) {
          const res = await getTableReplay(tableId, { fromSeq, limit: 500 })
          const chunk = (res.items || []).map(normalizeEvent)
          all.push(...chunk)
          if (!res.has_more || chunk.length === 0) break
          fromSeq = res.next_from_seq
          if (all.length > 10000) break
        }
        const tl = await getTableTimeline(tableId)
        if (dead) return
        setEvents(all)
        setTimeline(tl.items || [])
        setIndex(0)
        setPlaying(false)
        setStateOverride(null)
      } catch (err) {
        if (!dead) setError(err?.message || 'replay_load_failed')
      } finally {
        if (!dead) setLoading(false)
      }
    }
    load()
    return () => {
      dead = true
    }
  }, [tableId])

  useEffect(() => {
    if (!playing) return
    const interval = Math.max(80, 550 / speed)
    const id = setInterval(() => {
      setIndex((prev) => {
        if (prev >= events.length - 1) {
          setPlaying(false)
          return prev
        }
        return prev + 1
      })
    }, interval)
    return () => clearInterval(id)
  }, [playing, speed, events.length])

  const holeByHand = useMemo(() => buildHoleCardsByHand(events), [events])
  const currentEvent = events[index] || null
  const replayState = useMemo(() => stateAt(events, index, stateOverride, holeByHand), [events, index, stateOverride, holeByHand])
  const thoughtItems = useMemo(() => buildThoughtItems(events).slice(Math.max(0, index - 120), index + 1).reverse(), [events, index])
  const actionItems = useMemo(() => buildActionItems(events).slice(Math.max(0, index - 120), index + 1).reverse(), [events, index])
  const handResult = useMemo(() => handResultAt(events, index), [events, index])

  const jumpToSeq = async (seq) => {
    try {
      const snap = await getTableSnapshot(tableId, seq)
      const idx = events.findIndex((ev) => ev.global_seq >= seq)
      const snapshotState = {
        table_id: snap.state?.table_id,
        hand_id: snap.state?.hand_id,
        global_seq: seq,
        street: snap.state?.street,
        pot_cc: snap.state?.pot_cc ?? 0,
        board_cards: snap.state?.board_cards || [],
        current_actor_seat: snap.state?.current_actor_seat,
        seats: toSeatMap(snap.state || {})
      }
      const holes = holeByHand.get(snapshotState.hand_id) || new Map()
      for (const s of snapshotState.seats) {
        s.hole_cards = holes.get(s.agent_id) || null
      }
      setStateOverride(snapshotState)
      setIndex(idx >= 0 ? idx : events.length - 1)
      setPlaying(false)
    } catch (err) {
      setError(err?.message || 'snapshot_load_failed')
    }
  }

  const statusText = loading ? 'Loading replay...' : `event ${index + 1}/${events.length}`

  return (
    <section className="page replay-page">
      <div className="replay-shell-clean">
        <header className="replay-controls-clean">
          <button className="btn btn-primary" onClick={() => setPlaying((p) => !p)} disabled={events.length === 0}>
            {playing ? 'Pause' : 'Play'}
          </button>
          <button className="btn btn-ghost" onClick={() => setIndex((v) => Math.max(0, v - 1))} disabled={index <= 0}>Prev</button>
          <button className="btn btn-ghost" onClick={() => setIndex((v) => Math.min(events.length - 1, v + 1))} disabled={index >= events.length - 1}>Next</button>
          <label className="replay-speed-label">
            Speed
            <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
              <option value={0.5}>0.5x</option>
              <option value={1}>1x</option>
              <option value={2}>2x</option>
              <option value={4}>4x</option>
            </select>
          </label>
          <span className="replay-status muted">{statusText}</span>
          <span className="replay-table muted">table={tableId}</span>
          <Link className="btn btn-ghost" to="/history">Back to History</Link>
        </header>

        {error && <div className="replay-error muted">{error}</div>}

        <div className="replay-main-clean">
          <ReplayTableStage state={replayState} currentEvent={currentEvent?.payload || {}} handResult={handResult} />
          <aside className="replay-side-clean">
            <ReplayThoughtTimeline items={thoughtItems} activeSeq={currentEvent?.global_seq} />
            <ReplayActionTicker items={actionItems} activeSeq={currentEvent?.global_seq} />
          </aside>
        </div>

        <div className="replay-timeline-clean">
          <div className="replay-timeline-title">Hand Timeline</div>
          <div className="replay-timeline-list">
            {timeline.length === 0 && <div className="muted">No timeline</div>}
            {timeline.map((h) => (
              <button key={h.hand_id} className="replay-hand-node" onClick={() => jumpToSeq(h.start_seq || 1)}>
                <span className="replay-hand-id">{h.hand_id}</span>
                <span className="replay-hand-meta">winner={h.winner_agent_id || '-'} pot={h.pot_cc ?? '-'}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
