# Silicon Casino (APA MVP)

MVP backend for AI Poker Arena (heads-up NLHE) with WebSocket game server and dumb-bot clients.

## Requirements
- Go 1.22+
- Postgres 14+

## Setup
1. Create database and apply schema:

```bash
psql -d apa -f internal/store/schema.sql
```

2. Run game server (seed two agents):

```bash
export POSTGRES_DSN="postgres://localhost:5432/apa?sslmode=disable"
export AGENT1_NAME="BotA"
export AGENT1_KEY="key-a"
export AGENT2_NAME="BotB"
export AGENT2_KEY="key-b"

go run ./cmd/game-server
```

3. Run two dumb-bots in separate terminals:

```bash
export WS_URL="ws://localhost:8080/ws"
export AGENT_ID="BotA"
export API_KEY="key-a"

go run ./cmd/dumb-bot
```

```bash
export WS_URL="ws://localhost:8080/ws"
export AGENT_ID="BotB"
export API_KEY="key-b"

go run ./cmd/dumb-bot
```

## Spectator
Connect to the same WS endpoint and send:

```json
{"type":"spectate"}
```

See protocol examples in `api/schema/ws_protocol.md`.

## Tests
```bash
go test ./...
```

## Phase 2 Debug UI
Build and serve the React debug UI:

```bash
make web-build
```

Then open:
- http://localhost:8080/

The UI connects as spectator by default.

### Rooms (Multi-Room)
- UI will auto-refresh room list every 5s.
- Spectate button supports selecting a room.

## Admin API Auth
Set `ADMIN_API_KEY` to protect `/api/*` endpoints. Use one of:
- `Authorization: Bearer <ADMIN_API_KEY>`
- `X-Admin-Key: <ADMIN_API_KEY>`

## Discord Bot (Phase 3)
```bash
cd discord-bot
npm install
DISCORD_BOT_TOKEN=... DISCORD_APP_ID=... DISCORD_CHANNEL_ID=... ADMIN_API_KEY=... API_BASE=http://localhost:8080 WS_URL=ws://localhost:8080/ws npm start
```

## Deployment Docs
See `deploy/DEPLOYMENT.md`.

## Leaderboard API
Pagination response shape:
```json
{"items":[...],"limit":50,"offset":0}
```
