package public

import "time"

type RoomsResponse struct {
	Items []RoomItem `json:"items"`
}

type RoomItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MinBuyinCC   int64  `json:"min_buyin_cc"`
	SmallBlindCC int64  `json:"small_blind_cc"`
	BigBlindCC   int64  `json:"big_blind_cc"`
}

type TablesResponse struct {
	Items  []TableItem `json:"items"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type TableItem struct {
	TableID      string    `json:"table_id"`
	RoomID       string    `json:"room_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	SmallBlindCC int64     `json:"small_blind_cc"`
	BigBlindCC   int64     `json:"big_blind_cc"`
}

type TableHistoryResponse struct {
	Items  []TableHistoryItem `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

type TableHistoryItem struct {
	TableID       string     `json:"table_id"`
	RoomID        string     `json:"room_id"`
	Status        string     `json:"status"`
	SmallBlindCC  int64      `json:"small_blind_cc"`
	BigBlindCC    int64      `json:"big_blind_cc"`
	CreatedAt     time.Time  `json:"created_at"`
	LastHandEnded *time.Time `json:"last_hand_ended_at"`
}

type AgentTableResponse struct {
	AgentID string `json:"agent_id"`
	RoomID  string `json:"room_id"`
	TableID string `json:"table_id"`
}

type ReplayResponse struct {
	Items       []ReplayEvent `json:"items"`
	NextFromSeq int64         `json:"next_from_seq"`
	HasMore     bool          `json:"has_more"`
	LastSeq     int64         `json:"last_seq"`
}

type ReplayEvent struct {
	ID           string    `json:"id"`
	TableID      string    `json:"table_id"`
	HandID       string    `json:"hand_id"`
	GlobalSeq    int64     `json:"global_seq"`
	HandSeq      *int32    `json:"hand_seq"`
	EventType    string    `json:"event_type"`
	ActorAgentID string    `json:"actor_agent_id"`
	Payload      any       `json:"payload"`
	SchemaVer    int32     `json:"schema_version"`
	CreatedAt    time.Time `json:"created_at"`
}

type TimelineResponse struct {
	TableID string         `json:"table_id"`
	Items   []TimelineItem `json:"items"`
}

type TimelineItem struct {
	HandID        string     `json:"hand_id"`
	StartSeq      int64      `json:"start_seq"`
	EndSeq        int64      `json:"end_seq"`
	WinnerAgentID string     `json:"winner_agent_id"`
	PotCC         *int64     `json:"pot_cc"`
	StreetEnd     string     `json:"street_end"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
}

type SnapshotResponse struct {
	TableID string         `json:"table_id"`
	AtSeq   int64          `json:"at_seq"`
	State   map[string]any `json:"state"`
	Hit     bool           `json:"-"`
}

type LeaderboardResponse struct {
	Items  []LeaderboardItem `json:"items"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type LeaderboardItem struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	NetCC   int64  `json:"net_cc"`
}
