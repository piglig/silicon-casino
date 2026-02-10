package store

import (
	"context"
	"testing"
	"time"
)

func TestPublicLeaderboardExcludesTopupAndRanksByPlayResults(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	a1 := mustCreateAgent(t, st, ctx, "A", "key-a", 200000)
	a2 := mustCreateAgent(t, st, ctx, "B", "key-b", 200000)

	roomID, err := st.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	tableID, err := st.CreateTable(ctx, roomID, "active", 50, 100)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = st.Credit(ctx, a1, 1_000_000, "topup_credit", "topup", NewID())
	if err != nil {
		t.Fatalf("topup a1: %v", err)
	}

	for i := 0; i < 220; i++ {
		if err := recordSettledHand(t, st, ctx, tableID, a2, a1, 100); err != nil {
			t.Fatalf("record hand %d: %v", i, err)
		}
	}

	lb, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "score",
		MinHands:  200,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list leaderboard: %v", err)
	}
	if len(lb) != 2 {
		t.Fatalf("expected 2 leaderboard entries, got %d", len(lb))
	}
	if lb[0].AgentID != a2 {
		t.Fatalf("expected %s first, got %s", a2, lb[0].AgentID)
	}
	if lb[0].NetCCFromPlay <= 0 {
		t.Fatalf("expected positive net_cc_from_play for winner, got %d", lb[0].NetCCFromPlay)
	}
	if lb[1].NetCCFromPlay >= 0 {
		t.Fatalf("expected negative net_cc_from_play for loser, got %d", lb[1].NetCCFromPlay)
	}
}

func TestPublicLeaderboardRespectsRoomScopeAndMinHands(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	a1 := mustCreateAgent(t, st, ctx, "A", "key-a", 200000)
	a2 := mustCreateAgent(t, st, ctx, "B", "key-b", 200000)

	lowRoomID, err := st.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		t.Fatalf("create low room: %v", err)
	}
	midRoomID, err := st.CreateRoom(ctx, "Mid", 5000, 100, 200)
	if err != nil {
		t.Fatalf("create mid room: %v", err)
	}
	lowTableID, err := st.CreateTable(ctx, lowRoomID, "active", 50, 100)
	if err != nil {
		t.Fatalf("create low table: %v", err)
	}
	midTableID, err := st.CreateTable(ctx, midRoomID, "active", 100, 200)
	if err != nil {
		t.Fatalf("create mid table: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := recordSettledHand(t, st, ctx, lowTableID, a1, a2, 100); err != nil {
			t.Fatalf("record low hand %d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := recordSettledHand(t, st, ctx, midTableID, a2, a1, 200); err != nil {
			t.Fatalf("record mid hand %d: %v", i, err)
		}
	}

	lbLow, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "low",
		SortBy:    "score",
		MinHands:  2,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list low leaderboard: %v", err)
	}
	if len(lbLow) < 2 || lbLow[0].AgentID != a1 {
		t.Fatalf("expected %s first in low scope", a1)
	}

	lbMid, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "mid",
		SortBy:    "score",
		MinHands:  2,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list mid leaderboard: %v", err)
	}
	if len(lbMid) < 2 || lbMid[0].AgentID != a2 {
		t.Fatalf("expected %s first in mid scope", a2)
	}

	lbThreshold, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "score",
		MinHands:  7,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list threshold leaderboard: %v", err)
	}
	if len(lbThreshold) != 0 {
		t.Fatalf("expected empty leaderboard with high min_hands, got %d", len(lbThreshold))
	}
}

func TestPublicLeaderboardRespectsWindowStart(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	a1 := mustCreateAgent(t, st, ctx, "A", "key-a", 200000)
	a2 := mustCreateAgent(t, st, ctx, "B", "key-b", 200000)
	roomID, err := st.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	tableID, err := st.CreateTable(ctx, roomID, "active", 50, 100)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < 4; i++ {
		if err := recordSettledHand(t, st, ctx, tableID, a1, a2, 100); err != nil {
			t.Fatalf("record hand %d: %v", i, err)
		}
	}

	future := time.Now().UTC().Add(1 * time.Hour)
	lbFuture, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		WindowStart: &future,
		RoomScope:   "all",
		SortBy:      "score",
		MinHands:    1,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list future-scoped leaderboard: %v", err)
	}
	if len(lbFuture) != 0 {
		t.Fatalf("expected no rows for future window_start, got %d", len(lbFuture))
	}

	lbAll, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "score",
		MinHands:  1,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list unscoped leaderboard: %v", err)
	}
	if len(lbAll) == 0 {
		t.Fatal("expected rows when window_start is nil")
	}
}

func TestPublicLeaderboardSortModes(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	a1 := mustCreateAgent(t, st, ctx, "A", "key-a", 400000)
	a2 := mustCreateAgent(t, st, ctx, "B", "key-b", 400000)
	a3 := mustCreateAgent(t, st, ctx, "C", "key-c", 400000)
	roomID, err := st.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	tableID, err := st.CreateTable(ctx, roomID, "active", 50, 100)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// A wins fewer but high-value hands.
	for i := 0; i < 5; i++ {
		if err := recordSettledHand(t, st, ctx, tableID, a1, a2, 400); err != nil {
			t.Fatalf("record A hand %d: %v", i, err)
		}
	}
	// C plays many hands with moderate win rate.
	for i := 0; i < 22; i++ {
		if err := recordSettledHand(t, st, ctx, tableID, a3, a2, 100); err != nil {
			t.Fatalf("record C win hand %d: %v", i, err)
		}
	}
	for i := 0; i < 18; i++ {
		if err := recordSettledHand(t, st, ctx, tableID, a2, a3, 100); err != nil {
			t.Fatalf("record C loss hand %d: %v", i, err)
		}
	}

	lbNet, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "net_cc_from_play",
		MinHands:  1,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list net leaderboard: %v", err)
	}
	if len(lbNet) == 0 || lbNet[0].AgentID != a1 {
		t.Fatalf("expected %s first by net_cc_from_play", a1)
	}

	lbHands, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "hands_played",
		MinHands:  1,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list hands leaderboard: %v", err)
	}
	if len(lbHands) == 0 || lbHands[0].AgentID != a2 {
		t.Fatalf("expected %s first by hands_played", a2)
	}

	lbWinRate, err := st.ListLeaderboard(ctx, LeaderboardFilter{
		RoomScope: "all",
		SortBy:    "win_rate",
		MinHands:  1,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list win_rate leaderboard: %v", err)
	}
	if len(lbWinRate) == 0 || lbWinRate[0].AgentID != a1 {
		t.Fatalf("expected %s first by win_rate", a1)
	}
}

func recordSettledHand(t *testing.T, st *Store, ctx context.Context, tableID, winnerID, loserID string, amount int64) error {
	t.Helper()

	handID, err := st.CreateHand(ctx, tableID)
	if err != nil {
		return err
	}
	if _, err := st.Debit(ctx, loserID, amount, "bet_debit", "hand", handID); err != nil {
		return err
	}
	if _, err := st.Credit(ctx, winnerID, amount, "pot_credit", "hand", handID); err != nil {
		return err
	}
	pot := amount * 2
	return st.EndHandWithSummary(ctx, handID, winnerID, &pot, "showdown")
}
