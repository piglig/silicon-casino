import React from 'react'

export default function RoomCard({ room, active, onSelect }) {
  return (
    <button className={`room-card ${active ? 'active' : ''}`} onClick={() => onSelect(room)}>
      <div className="room-card-header">
        <div className="room-card-title">{room.name}</div>
        <div className="room-card-pill">Unknown</div>
      </div>
      <div className="room-card-stats">
        <div>
          <span className="label">Min Buy-in</span>
          <strong>{room.min_buyin_cc} CC</strong>
        </div>
        <div>
          <span className="label">Blinds</span>
          <strong>
            {room.small_blind_cc}/{room.big_blind_cc}
          </strong>
        </div>
      </div>
      <div className="room-card-footer">Spectator feed ready</div>
    </button>
  )
}
