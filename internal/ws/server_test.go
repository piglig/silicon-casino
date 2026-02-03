package ws

import (
	"context"
	"testing"

	"silicon-casino/internal/ledger"
	"silicon-casino/internal/testutil"
)

func TestSelectRoomInsufficientBuyin(t *testing.T) {
	st, cleanup := testutil.OpenTestStore(t)
	defer cleanup()

	ctx := context.Background()
	roomID, err := st.CreateRoom(ctx, "Test", 5000, 50, 100)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	agentID, err := st.CreateAgent(ctx, "AgentA", "key-a")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := st.EnsureAccount(ctx, agentID, 1000); err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	agent, err := st.GetAgentByAPIKey(ctx, "key-a")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	srv := NewServer(st, ledger.New(st))
	client := &Client{agent: agent}
	room, code := srv.selectRoom(ctx, client, JoinMessage{JoinMode: "select", RoomID: roomID})
	if room != nil || code != "insufficient_buyin" {
		t.Fatalf("expected insufficient_buyin, got room=%v code=%s", room, code)
	}
}
