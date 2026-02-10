package public

import (
	"testing"
	"time"
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
