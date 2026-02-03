package ws

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"silicon-casino/internal/game"
	"silicon-casino/internal/ledger"
	"silicon-casino/internal/store"
)

type Client struct {
	conn         *websocket.Conn
	send         chan []byte
	role         string
	playerIdx    int
	agent        *store.Agent
	session      *TableSession
	spectateRoom string
}

type TableSession struct {
	id       string
	room     *store.Room
	engine   *game.Engine
	players  [2]*Client
	actionCh chan ActionEnvelope
	done     chan struct{}
}

type Server struct {
	store       *store.Store
	ledger      *ledger.Ledger
	upgrader    websocket.Upgrader
	mu          sync.Mutex
	spectators  map[*Client]bool
	waiting     map[string]*Client
	sessions    map[string]*TableSession
	byAgentID   map[string]*Client
	metricsMu   sync.Mutex
	handsPlayed int64
	timeouts    int64
	folds       int64
}

func NewServer(store *store.Store, ledger *ledger.Ledger) *Server {
	return &Server{
		store:      store,
		ledger:     ledger,
		upgrader:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		spectators: map[*Client]bool{},
		waiting:    map[string]*Client{},
		sessions:   map[string]*TableSession{},
		byAgentID:  map[string]*Client{},
	}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 8), role: ""}

	go s.writeLoop(client)
	s.readLoop(client)
}

func (s *Server) readLoop(c *Client) {
	defer func() {
		s.unregister(c)
		_ = c.conn.Close()
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &base); err != nil {
			continue
		}
		switch base.Type {
		case "join":
			if c.role != "" {
				continue
			}
			var join JoinMessage
			if err := json.Unmarshal(msg, &join); err != nil {
				continue
			}
			s.handleJoin(c, join)
		case "spectate":
			if c.role != "" {
				continue
			}
			var spec SpectateMessage
			_ = json.Unmarshal(msg, &spec)
			c.role = "spectator"
			c.spectateRoom = spec.RoomID
			s.mu.Lock()
			s.spectators[c] = true
			s.mu.Unlock()
		case "action":
			if c.role != "player" || c.session == nil {
				continue
			}
			var action ActionMessage
			if err := json.Unmarshal(msg, &action); err != nil {
				continue
			}
			a := game.Action{Player: c.playerIdx, Type: game.ActionType(action.Action), Amount: action.Amount}
			c.session.actionCh <- ActionEnvelope{Player: c.playerIdx, Action: a, Log: action.ThoughtLog}
		}
	}
}

func (s *Server) writeLoop(c *Client) {
	for msg := range c.send {
		_ = c.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (s *Server) handleJoin(c *Client, join JoinMessage) {
	agent, err := s.store.GetAgentByAPIKey(context.Background(), join.APIKey)
	if err != nil {
		s.sendJoinResult(c, false, "invalid_action", "")
		return
	}
	c.agent = agent

	room, code := s.selectRoom(context.Background(), c, join)
	if room == nil {
		s.sendJoinResult(c, false, code, "")
		return
	}

	s.sendJoinResult(c, true, "", room.ID)
	s.enqueueOrMatch(c, room)
}

func (s *Server) selectRoom(ctx context.Context, c *Client, join JoinMessage) (*store.Room, string) {
	balance, err := s.store.GetAccountBalance(ctx, c.agent.ID)
	if err != nil {
		return nil, "invalid_action"
	}
	mode := strings.ToLower(join.JoinMode)
	if mode == "" {
		mode = "random"
	}
	if mode == "select" {
		room, err := s.store.GetRoom(ctx, join.RoomID)
		if err != nil || room.Status != "active" {
			return nil, "room_not_found"
		}
		if balance < room.MinBuyinCC {
			return nil, "insufficient_buyin"
		}
		return room, ""
	}

	rooms, err := s.store.ListRooms(ctx)
	if err != nil {
		return nil, "no_available_room"
	}
	eligible := []store.Room{}
	for _, r := range rooms {
		if balance >= r.MinBuyinCC {
			eligible = append(eligible, r)
		}
	}
	if len(eligible) == 0 {
		return nil, "no_available_room"
	}
	pick := eligible[rand.Intn(len(eligible))]
	return &pick, ""
}

func (s *Server) enqueueOrMatch(c *Client, room *store.Room) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if old := s.byAgentID[c.agent.ID]; old != nil && old != c {
		delete(s.spectators, old)
		if old.role == "player" && old.session != nil {
			// let session handle disconnect
			old.session = nil
		}
		close(old.send)
		_ = old.conn.Close()
	}
	c.role = "player"
	s.byAgentID[c.agent.ID] = c

	if waiter := s.waiting[room.ID]; waiter != nil {
		delete(s.waiting, room.ID)
		s.createSession(room, waiter, c)
		return
	}
	s.waiting[room.ID] = c
}

func (s *Server) createSession(room *store.Room, c1, c2 *Client) {
	ctx := context.Background()
	tableID, err := s.store.CreateTable(ctx, room.ID, "active", room.SmallBlindCC, room.BigBlindCC)
	if err != nil {
		return
	}
	engine := game.NewEngine(s.store, s.ledger, tableID, room.SmallBlindCC, room.BigBlindCC)
	session := &TableSession{
		id:       tableID,
		room:     room,
		engine:   engine,
		players:  [2]*Client{c1, c2},
		actionCh: make(chan ActionEnvelope, 16),
		done:     make(chan struct{}),
	}
	c1.playerIdx = 0
	c2.playerIdx = 1
	c1.session = session
	c2.session = session
	s.sessions[tableID] = session

	go session.run(s)
}

func (s *Server) unregister(c *Client) {
	s.mu.Lock()
	if c.role == "player" {
		if c.session != nil {
			// force fold in session
			session := c.session
			idx := c.playerIdx
			s.mu.Unlock()
			select {
			case session.actionCh <- ActionEnvelope{Player: idx, Action: game.Action{Player: idx, Type: game.ActionFold}}:
			default:
			}
			s.mu.Lock()
		}
		if c.agent != nil && s.byAgentID[c.agent.ID] == c {
			delete(s.byAgentID, c.agent.ID)
		}
		for roomID, waiter := range s.waiting {
			if waiter == c {
				delete(s.waiting, roomID)
			}
		}
	}
	if c.role == "spectator" {
		delete(s.spectators, c)
	}
	s.mu.Unlock()
	close(c.send)
}

func (s *Server) sendJoinResult(c *Client, ok bool, errCode, roomID string) {
	msg, _ := json.Marshal(JoinResult{Type: "join_result", ProtocolVersion: game.ProtocolVersion, Ok: ok, Error: errCode, RoomID: roomID})
	c.send <- msg
}

func (ts *TableSession) run(s *Server) {
	ctx := context.Background()
	for {
		p1 := ts.players[0]
		p2 := ts.players[1]
		if p1 == nil || p2 == nil {
			return
		}
		players := [2]*game.Player{
			{ID: p1.agent.ID, Name: p1.agent.Name, Seat: 0},
			{ID: p2.agent.ID, Name: p2.agent.Name, Seat: 1},
		}
		started := time.Now()
		if err := ts.engine.StartHand(ctx, players[0], players[1], ts.room.SmallBlindCC, ts.room.BigBlindCC); err != nil {
			log.Println("start hand error", err)
			return
		}
		log.Println("hand_start", ts.engine.State.HandID, "room", ts.room.Name)

		handOver := false
		for !handOver {
			ts.broadcastState(s)
			actor := ts.engine.State.CurrentActor
			timeout := time.NewTimer(ts.engine.State.ActionTimeout)
			select {
			case env := <-ts.actionCh:
				if env.Player != actor {
					break
				}
				log.Println("action_received", env.Action.Type, "player", env.Player)
				s.broadcastEventLog(actor, env.Action.Type, env.Action.Amount, env.Log, "action", s)
				done, err := ts.engine.ApplyAction(ctx, env.Action)
				if err != nil {
					ts.sendActionResult(actor, false, mapError(err))
					break
				}
				ts.sendActionResult(actor, true, "")
				if done {
					handOver = ts.handleRoundEnd(ctx, s)
				}
			case <-timeout.C:
				_, _ = ts.engine.ApplyAction(ctx, game.Action{Player: actor, Type: game.ActionFold})
				s.metricsIncTimeouts(s)
				s.broadcastEventLog(actor, game.ActionFold, 0, "", "timeout", s)
				handOver = ts.handleRoundEnd(ctx, s)
			}
			timeout.Stop()
		}
		_ = s.store.EndHand(ctx, ts.engine.State.HandID)
		ts.metricsHandEnd(s, started)
	}
}

func (ts *TableSession) handleRoundEnd(ctx context.Context, s *Server) bool {
	st := ts.engine.State
	if st.Players[0].Folded || st.Players[1].Folded {
		winner, _ := ts.engine.Settle(ctx)
		ts.broadcastHandEnd(winner, s)
		return true
	}
	if st.Players[0].AllIn || st.Players[1].AllIn {
		ts.engine.FastForwardToShowdown()
		winner, _ := ts.engine.Settle(ctx)
		ts.broadcastHandEnd(winner, s)
		return true
	}
	if st.Street == game.StreetRiver {
		winner, _ := ts.engine.Settle(ctx)
		ts.broadcastHandEnd(winner, s)
		return true
	}
	ts.engine.NextStreet()
	return false
}

func (ts *TableSession) broadcastState(s *Server) {
	p0 := ts.players[0]
	p1 := ts.players[1]
	if p0 == nil || p1 == nil {
		return
	}
	msg0, _ := json.Marshal(ts.engine.State.SnapshotFor(0, true))
	msg1, _ := json.Marshal(ts.engine.State.SnapshotFor(1, true))
	msg0Public, _ := json.Marshal(ts.engine.State.SnapshotFor(0, false))

	p0.send <- msg0
	p1.send <- msg1

	s.mu.Lock()
	for c := range s.spectators {
		if c != nil {
			if c.spectateRoom == "" || c.spectateRoom == ts.room.ID {
				c.send <- msg0Public
			}
		}
	}
	s.mu.Unlock()
	_ = msg1
}

func (ts *TableSession) broadcastHandEnd(winner string, s *Server) {
	balances := []BalanceInfo{}
	for i := 0; i < 2; i++ {
		p := ts.engine.State.Players[i]
		balances = append(balances, BalanceInfo{AgentID: p.ID, Balance: p.Stack})
	}
	msgPlayers, _ := json.Marshal(HandEnd{Type: "hand_end", ProtocolVersion: game.ProtocolVersion, Winner: winner, Pot: ts.engine.State.Pot, Balances: balances})
	for _, p := range ts.players {
		if p != nil {
			p.send <- msgPlayers
		}
	}
	showdown := []ShowdownHand{}
	for i := 0; i < 2; i++ {
		p := ts.engine.State.Players[i]
		cards := []string{}
		for _, c := range p.Hole {
			cards = append(cards, c.String())
		}
		showdown = append(showdown, ShowdownHand{AgentID: p.ID, HoleCards: cards})
	}
	msgSpectators, _ := json.Marshal(HandEnd{Type: "hand_end", ProtocolVersion: game.ProtocolVersion, Winner: winner, Pot: ts.engine.State.Pot, Balances: balances, Showdown: showdown})
	s.mu.Lock()
	for c := range s.spectators {
		if c.spectateRoom == "" || c.spectateRoom == ts.room.ID {
			c.send <- msgSpectators
		}
	}
	s.mu.Unlock()
}

func (ts *TableSession) sendActionResult(playerIdx int, ok bool, errStr string) {
	p := ts.players[playerIdx]
	if p == nil {
		return
	}
	msg, _ := json.Marshal(ActionResult{Type: "action_result", ProtocolVersion: game.ProtocolVersion, Ok: ok, Error: errStr})
	p.send <- msg
}

func (ts *TableSession) broadcastEventLog(playerIdx int, action game.ActionType, amount int64, thoughtLog, event string, s *Server) {
	msg, _ := json.Marshal(EventLog{
		Type:            "event_log",
		ProtocolVersion: game.ProtocolVersion,
		TimestampMS:     time.Now().UnixMilli(),
		PlayerSeat:      ts.engine.State.Players[playerIdx].Seat,
		Action:          string(action),
		Amount:          amount,
		ThoughtLog:      thoughtLog,
		Event:           event,
	})
	for _, p := range ts.players {
		if p != nil {
			p.send <- msg
		}
	}
	s.mu.Lock()
	for c := range s.spectators {
		if c.spectateRoom == "" || c.spectateRoom == ts.room.ID {
			c.send <- msg
		}
	}
	s.mu.Unlock()
}

func (ts *TableSession) metricsIncTimeouts(s *Server) {
	s.metricsMu.Lock()
	s.timeouts++
	s.folds++
	s.metricsMu.Unlock()
}

func (ts *TableSession) metricsHandEnd(s *Server, started time.Time) {
	duration := time.Since(started)
	s.metricsMu.Lock()
	s.handsPlayed++
	log.Println("hand_end", ts.engine.State.HandID, "duration_ms", duration.Milliseconds(), "hands_played", s.handsPlayed, "timeouts", s.timeouts, "folds", s.folds)
	s.metricsMu.Unlock()
}

func mapError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid_raise") {
		return "invalid_raise"
	}
	if strings.Contains(msg, "invalid_action") {
		return "invalid_action"
	}
	if strings.Contains(msg, "not_your_turn") {
		return "not_your_turn"
	}
	if strings.Contains(msg, "insufficient_balance") {
		return "insufficient_balance"
	}
	return "unknown_error"
}
