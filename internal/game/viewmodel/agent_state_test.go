package viewmodel

import (
	"testing"
	"time"

	"silicon-casino/internal/game"
)

func TestBuildAgentStateVisibilityAndSeatData(t *testing.T) {
	st := &game.TableState{
		HandID:        "hand_1",
		Street:        game.StreetFlop,
		Pot:           900,
		Community:     []game.Card{{Rank: game.Ace, Suit: game.Spades}, {Rank: game.King, Suit: game.Hearts}, {Rank: game.Queen, Suit: game.Diamonds}},
		CurrentActor:  1,
		CurrentBet:    400,
		RoundBets:     [2]int64{200, 400},
		ActionTimeout: 5 * time.Second,
		Players: [2]*game.Player{
			{ID: "a1", Seat: 0, Stack: 1000, Hole: []game.Card{{Rank: game.Two, Suit: game.Clubs}, {Rank: game.Three, Suit: game.Clubs}}, LastAction: game.ActionCall},
			{ID: "a2", Seat: 1, Stack: 800, Hole: []game.Card{{Rank: game.Four, Suit: game.Clubs}, {Rank: game.Five, Suit: game.Clubs}}, LastAction: game.ActionRaise},
		},
	}

	view := BuildAgentState(st, 0, "turn_1", false)
	if len(view.MyHoleCards) != 2 {
		t.Fatalf("expected 2 own hole cards, got %d", len(view.MyHoleCards))
	}
	if len(view.CommunityCards) != 3 {
		t.Fatalf("expected 3 community cards, got %d", len(view.CommunityCards))
	}
	if len(view.Seats) != 2 {
		t.Fatalf("expected 2 seats, got %d", len(view.Seats))
	}
	if view.Seats[0].StreetContribution != 200 {
		t.Fatalf("expected seat0 contribution 200, got %d", view.Seats[0].StreetContribution)
	}
	if view.Seats[0].ToCall != 200 {
		t.Fatalf("expected seat0 to_call 200, got %d", view.Seats[0].ToCall)
	}
	if view.Seats[1].ToCall != 0 {
		t.Fatalf("expected seat1 to_call 0, got %d", view.Seats[1].ToCall)
	}
}
