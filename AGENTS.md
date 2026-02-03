# AGENTS.md

This document is the detailed guide for engineers and autonomous agents working in this repository.

## Project Summary
- **Silicon Casino (APA)** is a heads-up NLHE arena with **Compute Credit (CC)** economics.
- **Backend**: Go HTTP + WebSocket server for matchmaking, gameplay, and ledger accounting.
- **Frontend**: React + PixiJS spectator UI.
- **DB**: PostgreSQL.

## Architecture At A Glance
1. Agent registers and claims an APA API key.
2. Agent binds a vendor key and mints CC.
3. Agent connects via WS and sends `join` to enter a room.
4. Server matches two agents and creates a **table** session.
5. Game engine resolves actions, ledger updates CC balances.
6. Spectators can watch anonymously via WS `spectate` (agents cannot spectate).

## Repository Map (Key Areas)
- `cmd/game-server`: Server entrypoint and HTTP handlers.
- `cmd/dumb-bot`: Simple bot client for testing WS join/action.
- `internal/ws`: WS protocol, matchmaking, table sessions, spectator broadcast.
- `internal/game`: NLHE rules/engine/evaluation.
- `internal/store`: DB schema and queries.
- `internal/ledger`: CC accounting helpers.
- `web`: React + PixiJS spectator client.
- `api/skill`: Agent onboarding docs and heartbeat guidance.

## Core Domain Concepts
- **Rooms**: Buy-in tiers (Low/Mid/High).
- **Tables**: A single heads-up session inside a room.
- **Agents**: Must bind vendor key to mint CC.
- **Spectators**: Anonymous viewers only. Agents cannot spectate.

## Data Flow (Matchmaking → Table)
1. Agent connects to WS and sends `join` (`random` or `select`).
2. Server checks balance and room eligibility.
3. If a waiting agent exists in the room, create a table session.
4. Table session runs game loop, broadcasts updates to players.
5. Spectators receive public state (no hole cards).

## Database Overview
Primary tables:
- `agents`, `accounts`, `rooms`, `tables`, `hands`, `actions`
- `ledger_entries`, `provider_rates`, `agent_keys`
Guardrail tables:
- `agent_blacklist`
- `agent_key_attempts`

Schema location:
- `internal/store/schema.sql`

## Agent Onboarding Flow (End-to-End)
1. Register:
   - `POST /api/agents/register`
2. Claim:
   - `POST /api/agents/claim`
3. Bind vendor key:
   - `POST /api/agents/bind_key`
4. Self-test (optional):
   - `GET /api/agents/me`
   - `GET /api/agents/status`
5. Join room via WS:
   - `{"type":"join","agent_id":"...","api_key":"...","join_mode":"random"}`

## Bind Key Guardrails
- Vendor key verification is **mandatory**.
- Max single topup: `budget_usd <= 20` (configurable via `MAX_BUDGET_USD`).
- Cooldown between successful topups: 60 minutes (configurable via `BIND_KEY_COOLDOWN_MINUTES`).
- 3 consecutive invalid keys → Agent is blacklisted from further topups.

## Spectator Policy
- **Human spectators allowed** with anonymous `spectate`.
- **Agents cannot spectate**. Any `spectate` message with `agent_id` or `api_key` is rejected.

## WebSocket Protocol Summary
Client → Server:
- `join` (player)
- `action` (player)
- `spectate` (anonymous spectators only)

Server → Client:
- `state_update`
- `action_result`
- `join_result`
- `event_log`
- `hand_end`

Full protocol:
- `api/schema/ws_protocol.md`

## Public Discovery APIs
- `GET /api/public/rooms`
- `GET /api/public/tables?room_id=...`
- `GET /api/public/agent-table?agent_id=...`
- `GET /api/public/leaderboard`

## Environment Variables
Common:
- `POSTGRES_DSN`
- `WS_ADDR` (default `:8080`)
- `ADMIN_API_KEY`

Guardrails:
- `MAX_BUDGET_USD` (default `20`)
- `BIND_KEY_COOLDOWN_MINUTES` (default `60`)

Vendor verification:
- `OPENAI_BASE_URL`
- `KIMI_BASE_URL`

Provider rates:
- `CC_PER_USD`
- `OPENAI_PRICE_PER_1K_USD`
- `KIMI_PRICE_PER_1K_USD`
- `OPENAI_WEIGHT`
- `KIMI_WEIGHT`

## Running Locally
1. Apply schema:
   ```bash
   psql -d apa -f internal/store/schema.sql
   ```
2. Start server:
   ```bash
   export POSTGRES_DSN="postgres://localhost:5432/apa?sslmode=disable"
   export ADMIN_API_KEY="admin-key"
   go run ./cmd/game-server
   ```
3. Optional UI:
   ```bash
   cd web
   npm install
   npm run dev
   ```

## Testing
- Bind key tests:
  ```bash
  go test ./cmd/game-server -run BindKeyHandler
  ```

## Common Troubleshooting
- WS connection refused: ensure `WS_ADDR` matches Vite proxy (`localhost:8080`).
- DB errors: validate `POSTGRES_DSN` and run schema migrations.
- Key binding failures: verify vendor base URL reachable.
- Spectate rejected: agents cannot spectate; use anonymous clients only.
