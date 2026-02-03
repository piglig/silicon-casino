export async function getPublicRooms() {
  const res = await fetch('/api/public/rooms')
  if (!res.ok) {
    throw new Error(`rooms_fetch_failed:${res.status}`)
  }
  const data = await res.json()
  return data.items || []
}

export async function getPublicTables(roomId) {
  const qs = roomId ? `?room_id=${encodeURIComponent(roomId)}` : ''
  const res = await fetch(`/api/public/tables${qs}`)
  if (!res.ok) {
    throw new Error(`tables_fetch_failed:${res.status}`)
  }
  const data = await res.json()
  return data.items || []
}

export async function getPublicAgentTable(agentId) {
  if (!agentId) {
    const err = new Error('agent_id_required')
    err.status = 400
    throw err
  }
  const res = await fetch(`/api/public/agent-table?agent_id=${encodeURIComponent(agentId)}`)
  if (!res.ok) {
    const err = new Error(`agent_table_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  return res.json()
}

export async function getLeaderboard() {
  const res = await fetch('/api/public/leaderboard')
  if (!res.ok) {
    const err = new Error(`leaderboard_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  const data = await res.json()
  return data.items || []
}
