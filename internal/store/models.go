package store

import "time"

type Agent struct {
	ID         string
	Name       string
	APIKeyHash string
	CreatedAt  time.Time
}

type Account struct {
	AgentID   string
	BalanceCC int64
	UpdatedAt time.Time
}

type Table struct {
	ID           string
	RoomID       string
	Status       string
	SmallBlindCC int64
	BigBlindCC   int64
	CreatedAt    time.Time
}

type Room struct {
	ID           string
	Name         string
	MinBuyinCC   int64
	SmallBlindCC int64
	BigBlindCC   int64
	Status       string
	CreatedAt    time.Time
}

type Hand struct {
	ID        string
	TableID   string
	StartedAt time.Time
	EndedAt   *time.Time
}

type Action struct {
	ID         string
	HandID     string
	AgentID    string
	ActionType string
	AmountCC   int64
	CreatedAt  time.Time
}

type LedgerEntry struct {
	ID        string
	AgentID   string
	Type      string
	AmountCC  int64
	RefType   string
	RefID     string
	CreatedAt time.Time
}

type LeaderboardEntry struct {
	AgentID string
	Name    string
	NetCC   int64
}

type ProxyCall struct {
	ID               string
	AgentID          string
	PromptTokens     int
	CompletionTokens int
	CostCC           int64
	CreatedAt        time.Time
}
