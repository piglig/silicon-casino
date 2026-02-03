const defaultUrl = (window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host + '/ws'

export class SpectateWS {
  constructor({ url = defaultUrl, onMessage, onStatus }) {
    this.url = url
    this.onMessage = onMessage
    this.onStatus = onStatus
    this.ws = null
    this.roomId = ''
    this.retry = 0
    this.closedByUser = false
    this.timer = null
  }

  connect(roomId) {
    this.roomId = roomId || ''
    this.closedByUser = false
    this.retry = 0
    this._open()
  }

  disconnect() {
    this.closedByUser = true
    this._clearTimer()
    if (this.ws) {
      this.ws.close()
    }
    this.ws = null
    this._emitStatus('disconnected')
  }

  _open() {
    this._emitStatus('connecting')
    const ws = new WebSocket(this.url)
    this.ws = ws

    ws.onopen = () => {
      this.retry = 0
      this._emitStatus('connected')
      ws.send(JSON.stringify({ type: 'spectate', room_id: this.roomId || undefined }))
    }

    ws.onclose = () => {
      this.ws = null
      if (this.closedByUser) {
        this._emitStatus('disconnected')
        return
      }
      this._emitStatus('reconnecting')
      this._scheduleReconnect()
    }

    ws.onerror = () => {
      this._emitStatus('error')
    }

    ws.onmessage = (ev) => {
      if (!this.onMessage) return
      try {
        const msg = JSON.parse(ev.data)
        this.onMessage(msg)
      } catch (err) {
        this.onMessage({ type: 'parse_error', error: err?.message || 'parse_error' })
      }
    }
  }

  _scheduleReconnect() {
    this._clearTimer()
    const delays = [500, 1000, 2000, 5000]
    const wait = delays[Math.min(this.retry, delays.length - 1)]
    this.retry += 1
    this.timer = setTimeout(() => this._open(), wait)
  }

  _clearTimer() {
    if (this.timer) {
      clearTimeout(this.timer)
      this.timer = null
    }
  }

  _emitStatus(status) {
    if (this.onStatus) this.onStatus(status)
  }
}
