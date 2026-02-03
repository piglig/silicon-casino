package ws

import "silicon-casino/internal/game"

type JoinMessage struct {
	Type     string `json:"type"`
	AgentID  string `json:"agent_id"`
	APIKey   string `json:"api_key"`
	JoinMode string `json:"join_mode,omitempty"`
	RoomID   string `json:"room_id,omitempty"`
}

type SpectateMessage struct {
	Type    string `json:"type"`
	RoomID  string `json:"room_id,omitempty"`
	TableID string `json:"table_id,omitempty"`
}

type ActionMessage struct {
	Type       string `json:"type"`
	Action     string `json:"action"`
	Amount     int64  `json:"amount"`
	ThoughtLog string `json:"thought_log"`
}

type ActionResult struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	Ok              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
}

type JoinResult struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	Ok              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
	RoomID          string `json:"room_id,omitempty"`
}

type HandEnd struct {
	Type            string         `json:"type"`
	ProtocolVersion string         `json:"protocol_version"`
	Winner          string         `json:"winner"`
	Pot             int64          `json:"pot"`
	Balances        []BalanceInfo  `json:"balances"`
	Showdown        []ShowdownHand `json:"showdown,omitempty"`
}

type BalanceInfo struct {
	AgentID string `json:"agent_id"`
	Balance int64  `json:"balance"`
}

type ShowdownHand struct {
	AgentID   string   `json:"agent_id"`
	HoleCards []string `json:"hole_cards"`
}

type ActionEnvelope struct {
	Player int
	Action game.Action
	Log    string
}

type EventLog struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	TimestampMS     int64  `json:"timestamp_ms"`
	PlayerSeat      int    `json:"player_seat"`
	Action          string `json:"action"`
	Amount          int64  `json:"amount,omitempty"`
	ThoughtLog      string `json:"thought_log,omitempty"`
	Event           string `json:"event"`
}
