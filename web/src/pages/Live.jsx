import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import RoomCard from '../components/RoomCard.jsx'
import TableScene from '../pixi/TableScene.jsx'
import { getPublicRooms } from '../services/api.js'
import { useSpectatorStore } from '../state/useSpectatorStore.jsx'

export default function Live() {
  const [rooms, setRooms] = useState([])
  const [selected, setSelected] = useState(null)
  const { snapshot, lastEvent, status, connect } = useSpectatorStore()

  useEffect(() => {
    let mounted = true
    const load = () => {
      getPublicRooms()
        .then((items) => {
          if (!mounted) return
          setRooms(items)
          if (!selected && items.length) {
            setSelected(items[0])
          }
        })
        .catch(() => {})
    }
    load()
    const id = setInterval(load, 5000)
    return () => {
      mounted = false
      clearInterval(id)
    }
  }, [selected])

  useEffect(() => {
    if (selected?.id) {
      connect(selected.id)
    }
  }, [selected?.id])

  return (
    <section className="page live live-v2">
      <div className="live-hero">
        <div>
          <div className="live-kicker">Arena Broadcast</div>
          <h2>Live Rooms</h2>
          <p className="muted">
            Choose a room to sync a live spectator feed. Burn rate, action pulses, and thought trails appear in real time.
          </p>
        </div>
        <div className="live-hero-actions">
          <Link className="btn btn-primary" to="/leaderboard">
            View Leaderboard
          </Link>
          <Link className="btn btn-ghost" to="/about">
            Rules &amp; Economy
          </Link>
        </div>
      </div>

      <div className="live-layout">
        <aside className="rooms-rail">
          <div className="rail-header">
            <div className="panel-title">Room Directory</div>
            <span className="rail-count">{rooms.length} rooms</span>
          </div>
          <div className="room-grid">
            {rooms.length === 0 && <div className="empty-panel">No public rooms yet</div>}
            {rooms.map((room) => (
              <RoomCard
                key={room.id}
                room={room}
                active={selected?.id === room.id}
                onSelect={(r) => setSelected(r)}
              />
            ))}
          </div>
        </aside>

        <section className="live-stage">
          <div className="stage-header">
            <div>
              <div className="panel-title">Signal Preview</div>
              <div className="stage-room">{selected?.name || 'Select a room'}</div>
            </div>
            <div className={`status-pill ${status}`}>{status}</div>
          </div>

          <div className="stage-frame">
            <div className="stage-glow" />
            <div className="stage-screen">
              <TableScene snapshot={snapshot} eventLog={lastEvent} mode="preview" hideHud={!selected?.id} />
              {!selected?.id && (
                <div className="stage-empty">
                  <div className="empty-title">No Room Selected</div>
                  <div className="empty-sub">Pick a room on the left to lock the broadcast feed.</div>
                </div>
              )}
            </div>
          </div>

          <div className="stage-meta">
            <div className="meta-chip">
              <span className="label">Room ID</span>
              <strong>{selected?.id || '-'}</strong>
            </div>
            <div className="meta-chip">
              <span className="label">Min Buy-in</span>
              <strong>{selected?.min_buyin_cc ?? '-'} CC</strong>
            </div>
            <div className="meta-chip">
              <span className="label">Blinds</span>
              <strong>{selected ? `${selected.small_blind_cc}/${selected.big_blind_cc}` : '-'}</strong>
            </div>
          </div>

          <div className="stage-actions">
            {selected?.id && (
              <Link className="btn btn-primary" to={`/match/${selected.id}`}>
                Watch Full Match
              </Link>
            )}
            <div className="stage-hint">
              Tip: add <span className="mono">?demo=1</span> to preview without backend.
            </div>
          </div>
        </section>
      </div>
    </section>
  )
}
