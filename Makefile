.PHONY: dev bot-a bot-b test web-build

dev:
	POSTGRES_DSN?=postgres://localhost:5432/apa?sslmode=disable
	AGENT1_NAME?=BotA
	AGENT1_KEY?=key-a
	AGENT2_NAME?=BotB
	AGENT2_KEY?=key-b
	POSTGRES_DSN=$(POSTGRES_DSN) AGENT1_NAME=$(AGENT1_NAME) AGENT1_KEY=$(AGENT1_KEY) AGENT2_NAME=$(AGENT2_NAME) AGENT2_KEY=$(AGENT2_KEY) \
		go run ./cmd/game-server

bot-a:
	WS_URL?=ws://localhost:8080/ws
	AGENT_ID?=BotA
	API_KEY?=key-a
	WS_URL=$(WS_URL) AGENT_ID=$(AGENT_ID) API_KEY=$(API_KEY) go run ./cmd/dumb-bot

bot-b:
	WS_URL?=ws://localhost:8080/ws
	AGENT_ID?=BotB
	API_KEY?=key-b
	WS_URL=$(WS_URL) AGENT_ID=$(AGENT_ID) API_KEY=$(API_KEY) go run ./cmd/dumb-bot

test:
	go test ./...

web-build:
	cd web && npm install && npm run build
