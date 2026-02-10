package public

import (
	"testing"
	"time"

	"silicon-casino/internal/store"
	"silicon-casino/internal/testutil"
)

func TestClampLeaderboardPage(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		offset    int
		wantLimit int
		wantOK    bool
	}{
		{name: "default limit", limit: 0, offset: 0, wantLimit: 50, wantOK: true},
		{name: "explicit small limit", limit: 20, offset: 0, wantLimit: 20, wantOK: true},
		{name: "limit clipped at top100 boundary", limit: 10, offset: 95, wantLimit: 5, wantOK: true},
		{name: "limit exactly remaining", limit: 1, offset: 99, wantLimit: 1, wantOK: true},
		{name: "offset 100 rejected", limit: 10, offset: 100, wantLimit: 0, wantOK: false},
		{name: "offset beyond 100 rejected", limit: 10, offset: 150, wantLimit: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotOK := clampLeaderboardPage(tt.limit, tt.offset)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotLimit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", gotLimit, tt.wantLimit)
			}
		})
	}
}

func TestLeaderboardWindowStart(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		window    string
		wantNil   bool
		wantRange time.Duration
	}{
		{name: "7d", window: "7d", wantNil: false, wantRange: 7 * 24 * time.Hour},
		{name: "30d", window: "30d", wantNil: false, wantRange: 30 * 24 * time.Hour},
		{name: "all", window: "all", wantNil: true},
		{name: "unknown", window: "x", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leaderboardWindowStart(tt.window)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil time")
			}
			diff := now.Sub(*got)
			if diff < tt.wantRange-time.Minute || diff > tt.wantRange+time.Minute {
				t.Fatalf("diff=%v out of expected range around %v", diff, tt.wantRange)
			}
		})
	}
}

func TestTableHistoryIncludesTotalAndSupportsOffsetBeyondTotal(t *testing.T) {
	st, cleanup := testutil.OpenTestStore(t)
	defer cleanup()
	svc := NewService(st)
	ctx := t.Context()

	agentA, err := st.CreateAgent(ctx, "A", "api-a", "claim-a")
	if err != nil {
		t.Fatalf("create agent A: %v", err)
	}
	agentB, err := st.CreateAgent(ctx, "B", "api-b", "claim-b")
	if err != nil {
		t.Fatalf("create agent B: %v", err)
	}
	roomID, err := st.CreateRoom(ctx, "Mid", 5000, 100, 200)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	tableID, err := st.CreateTable(ctx, roomID, "closed", 100, 200)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := st.CreateAgentSession(ctx, store.AgentSession{
		ID:        store.NewID(),
		AgentID:   agentA,
		RoomID:    roomID,
		TableID:   tableID,
		JoinMode:  "random",
		Status:    "active",
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}); err != nil {
		t.Fatalf("create session A: %v", err)
	}
	if err := st.CreateAgentSession(ctx, store.AgentSession{
		ID:        store.NewID(),
		AgentID:   agentB,
		RoomID:    roomID,
		TableID:   tableID,
		JoinMode:  "random",
		Status:    "active",
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}); err != nil {
		t.Fatalf("create session B: %v", err)
	}

	resp, err := svc.TableHistory(ctx, roomID, "", 20, 0)
	if err != nil {
		t.Fatalf("table history: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total=1, got %d", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	resp2, err := svc.TableHistory(ctx, roomID, "", 20, 20)
	if err != nil {
		t.Fatalf("table history with high offset: %v", err)
	}
	if resp2.Total != 1 {
		t.Fatalf("expected total still 1, got %d", resp2.Total)
	}
	if len(resp2.Items) != 0 {
		t.Fatalf("expected empty page for high offset, got %d", len(resp2.Items))
	}
}
