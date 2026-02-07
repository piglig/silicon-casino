const modules = import.meta.glob('../assets/replay-pixel/cards/*.png', { eager: true, import: 'default' })

const byName = {}
const labelUrlCache = new Map()
for (const [path, url] of Object.entries(modules)) {
  const file = path.split('/').pop()
  byName[file] = url
}

export function cardKeyFromLabel(label) {
  if (!label || typeof label !== 'string') return null
  const raw = label.trim().toUpperCase()
  if (raw.length < 2) return null
  const suitMap = { S: 'S', H: 'H', D: 'D', C: 'C', '♠': 'S', '♥': 'H', '♦': 'D', '♣': 'C' }
  const suit = suitMap[raw.slice(-1)]
  if (!suit) return null
  const rankRaw = raw.slice(0, -1)
  const rankMap = { A: 'A', K: 'K', Q: 'Q', J: 'J', T: '10', '10': '10', '9': '9', '8': '8', '7': '7', '6': '6', '5': '5', '4': '4', '3': '3', '2': '2' }
  const rank = rankMap[rankRaw]
  if (!rank) return null
  return `${rank}${suit}`
}

export function cardImageUrl(label, fallback = 'BACK_1') {
  const cacheKey = `${String(label || '')}|${fallback}`
  const cached = labelUrlCache.get(cacheKey)
  if (cached !== undefined) return cached
  const key = cardKeyFromLabel(label)
  const url = (key && byName[`${key}.png`]) ? byName[`${key}.png`] : (byName[`${fallback}.png`] || '')
  labelUrlCache.set(cacheKey, url)
  return url
}

export function cardBack(index = 1) {
  const name = index === 2 ? 'BACK_2.png' : 'BACK_1.png'
  return byName[name] || ''
}

export function allCardImages() {
  return byName
}
