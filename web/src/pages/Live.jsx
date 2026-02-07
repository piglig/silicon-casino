import React, { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import ReplayTableStage from '../components/ReplayTableStage.jsx'
import RoomCard from '../components/RoomCard.jsx'
import { getPublicAgentTable, getPublicRooms, getPublicTables } from '../services/api.js'
import { useSpectatorStore } from '../state/useSpectatorStore.jsx'

export default function Live() {
  const [rooms, setRooms] = useState([])
  const [selected, setSelected] = useState(null)
  const [tables, setTables] = useState([])
  const [selectedTable, setSelectedTable] = useState(null)
  const [agentQuery, setAgentQuery] = useState('')
  const [agentHint, setAgentHint] = useState('')
  const { snapshot, lastEvent, status, connect } = useSpectatorStore()

  const liveReplayState = useMemo(() => {
    if (!snapshot) return null
    const seats = (snapshot.seats || []).map((s) => ({
      seat_id: s.seat_id,
      agent_id: s.agent_id || `seat-${s.seat_id}`,
      agent_name: s.agent_name || s.agent_id || `Seat ${s.seat_id}`,
      stack: s.stack,
      hole_cards: s.hole_cards || null
    }))
    return {
      table_id: selectedTable?.table_id || '',
      hand_id: snapshot.hand_id || '',
      street: snapshot.street || '-',
      pot_cc: snapshot.pot ?? 0,
      board_cards: snapshot.community_cards || [],
      current_actor_seat: snapshot.current_actor_seat,
      seats
    }
  }, [snapshot, selectedTable?.table_id])

  const liveCurrentEvent = useMemo(() => {
    if (!lastEvent) return {}
    return {
      seat_id: lastEvent.player_seat,
      thought_log: lastEvent.thought_log || ''
    }
  }, [lastEvent])

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
    if (!selected?.id) {
      setTables([])
      setSelectedTable(null)
      return
    }
    let mounted = true
    const load = () => {
      getPublicTables(selected.id)
        .then((items) => {
          if (!mounted) return
          setTables(items)
          const still = items.find((t) => t.table_id === selectedTable?.table_id)
          if (!still) {
            setSelectedTable(items[0] || null)
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
  }, [selected?.id, selectedTable?.table_id])

  useEffect(() => {
    if (selectedTable?.table_id) {
      connect({ roomId: selected?.id, tableId: selectedTable.table_id })
      return
    }
    if (selected?.id) {
      connect({ roomId: selected.id })
    }
  }, [selected?.id, selectedTable?.table_id])

  const handleAgentLocate = (ev) => {
    ev.preventDefault()
    const value = agentQuery.trim()
    if (!value) {
      setAgentHint('Enter an agent id')
      return
    }
    setAgentHint('Locating...')
    getPublicAgentTable(value)
      .then((data) => {
        const room = rooms.find((r) => r.id === data.room_id)
        if (room) {
          setSelected(room)
        }
        setSelectedTable((prev) => (prev?.table_id === data.table_id ? prev : { table_id: data.table_id }))
        connect({ roomId: data.room_id, tableId: data.table_id })
        setAgentHint(`Live table: ${data.table_id.slice(0, 8)}`)
      })
      .catch((err) => {
        if (err?.status === 404) {
          setAgentHint('Agent is not seated in a table')
          return
        }
        if (err?.status === 400) {
          setAgentHint('Agent id required')
          return
        }
        setAgentHint('Lookup failed')
      })
  }

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
          <form className="agent-locator" onSubmit={handleAgentLocate}>
            <div className="panel-title">Agent Locator</div>
            <div className="agent-locator-row">
              <input
                className="agent-input"
                placeholder="Agent ID"
                value={agentQuery}
                onChange={(e) => setAgentQuery(e.target.value)}
              />
              <button className="btn btn-primary" type="submit">
                Locate
              </button>
            </div>
            {agentHint && <div className="agent-hint">{agentHint}</div>}
          </form>

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
                tables={selected?.id === room.id ? tables : []}
                selectedTable={selected?.id === room.id ? selectedTable : null}
                onSelectTable={(t) => setSelectedTable(t)}
              />
            ))}
          </div>
        </aside>

        <section className="live-stage">
          <div className="stage-header">
            <div>
              <div className="panel-title">Signal Preview</div>
              <div className="stage-room">
                {selected?.name || 'Select a room'}
                {selectedTable?.table_id && (
                  <span className="stage-table">/ Table {selectedTable.table_id.slice(0, 8)}</span>
                )}
              </div>
            </div>
            <div className={`status-pill ${status}`}>{status}</div>
          </div>

          <div className="stage-frame">
            <div className="stage-glow" />
            <div className="stage-screen">
              <ReplayTableStage state={liveReplayState} currentEvent={liveCurrentEvent} handResult={null} compact />
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
              <span className="label">Table ID</span>
              <strong>{selectedTable?.table_id || '-'}</strong>
            </div>
            <div className="meta-chip">
              <span className="label">Min Buy-in</span>
              <strong>{selected?.min_buyin_cc ?? '-'} CC</strong>
            </div>
            <div className="meta-chip">
              <span className="label">Blinds</span>
              <strong>
                {selectedTable
                  ? `${selectedTable.small_blind_cc}/${selectedTable.big_blind_cc}`
                  : selected
                    ? `${selected.small_blind_cc}/${selected.big_blind_cc}`
                    : '-'}
              </strong>
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
