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

export async function getTableHistory({ roomId = '', agentId = '', limit = 50, offset = 0 } = {}) {
  const params = new URLSearchParams()
  if (roomId) params.set('room_id', roomId)
  if (agentId) params.set('agent_id', agentId)
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  const res = await fetch(`/api/public/tables/history?${params.toString()}`)
  if (!res.ok) {
    throw new Error(`table_history_unavailable:${res.status}`)
  }
  return res.json()
}

export async function getTableReplay(tableId, { fromSeq = 1, limit = 200 } = {}) {
  const params = new URLSearchParams()
  params.set('from_seq', String(fromSeq))
  params.set('limit', String(limit))
  const res = await fetch(`/api/public/tables/${encodeURIComponent(tableId)}/replay?${params.toString()}`)
  if (!res.ok) {
    const err = new Error(`table_replay_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  return res.json()
}

export async function getTableTimeline(tableId) {
  const res = await fetch(`/api/public/tables/${encodeURIComponent(tableId)}/timeline`)
  if (!res.ok) {
    const err = new Error(`table_timeline_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  return res.json()
}

export async function getTableSnapshot(tableId, atSeq) {
  const params = new URLSearchParams()
  params.set('at_seq', String(atSeq))
  const res = await fetch(`/api/public/tables/${encodeURIComponent(tableId)}/snapshot?${params.toString()}`)
  if (!res.ok) {
    const err = new Error(`table_snapshot_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  return res.json()
}

export async function getAgentTables(agentId, { limit = 50, offset = 0 } = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  const res = await fetch(`/api/public/agents/${encodeURIComponent(agentId)}/tables?${params.toString()}`)
  if (!res.ok) {
    const err = new Error(`agent_tables_unavailable:${res.status}`)
    err.status = res.status
    throw err
  }
  return res.json()
}
