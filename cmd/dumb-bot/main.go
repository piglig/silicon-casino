package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type Snapshot struct {
	Type       string `json:"type"`
	MinRaise   int64  `json:"min_raise"`
	CurrentBet int64  `json:"current_bet"`
	CallAmount int64  `json:"call_amount"`
}

type Join struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	APIKey  string `json:"api_key"`
}

type Action struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	Amount int64  `json:"amount,omitempty"`
}

func main() {
	wsURL := getenv("WS_URL", "ws://localhost:8080/ws")
	agentID := getenv("AGENT_ID", "bot")
	apiKey := getenv("API_KEY", "")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	join := Join{Type: "join", AgentID: agentID, APIKey: apiKey}
	msg, _ := json.Marshal(join)
	_ = conn.WriteMessage(websocket.TextMessage, msg)

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			continue
		}
		if base.Type != "state_update" {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		action := decide(rnd, snap)
		payload, _ := json.Marshal(action)
		_ = conn.WriteMessage(websocket.TextMessage, payload)
	}
}

func decide(rnd *rand.Rand, s Snapshot) Action {
	if s.CallAmount == 0 {
		// check or bet
		if rnd.Intn(2) == 0 {
			return Action{Type: "action", Action: "check"}
		}
		return Action{Type: "action", Action: "bet", Amount: s.MinRaise}
	}
	// call, fold, or raise
	r := rnd.Intn(3)
	if r == 0 {
		return Action{Type: "action", Action: "fold"}
	}
	if r == 1 {
		return Action{Type: "action", Action: "call"}
	}
	return Action{Type: "action", Action: "raise", Amount: s.CurrentBet + s.MinRaise}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
