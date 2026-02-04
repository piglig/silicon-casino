package store

import (
	"testing"
	"time"
)

func TestAgentSessionStoreCRUD(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	agentID := mustCreateAgent(t, st, ctx, "BotA", "key-a", 10000)
	roomID, err := st.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	seat := 0
	sess := AgentSession{
		ID:        NewID(),
		AgentID:   agentID,
		RoomID:    roomID,
		SeatID:    &seat,
		JoinMode:  "random",
		Status:    "waiting",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := st.CreateAgentSession(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := st.GetAgentSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.AgentID != agentID || got.RoomID != roomID {
		t.Fatalf("unexpected session: %+v", got)
	}

	tableID, err := st.CreateTable(ctx, roomID, "active", 50, 100)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := st.UpdateAgentSessionMatch(ctx, sess.ID, tableID, seat); err != nil {
		t.Fatalf("update match: %v", err)
	}

	got, err = st.GetAgentSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session after update: %v", err)
	}
	if got.TableID != tableID || got.Status != "active" {
		t.Fatalf("unexpected matched session: %+v", got)
	}

	if err := st.CloseAgentSession(ctx, sess.ID); err != nil {
		t.Fatalf("close session: %v", err)
	}
	got, err = st.GetAgentSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session after close: %v", err)
	}
	if got.Status != "closed" || got.ClosedAt == nil {
		t.Fatalf("session not closed: %+v", got)
	}
}
