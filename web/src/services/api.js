export async function getPublicRooms() {
  const res = await fetch('/api/public/rooms')
  if (!res.ok) {
    throw new Error(`rooms_fetch_failed:${res.status}`)
  }
  const data = await res.json()
  return data.items || []
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
