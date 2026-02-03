package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"silicon-casino/internal/ledger"
	"silicon-casino/internal/store"
	"silicon-casino/internal/ws"
)

func main() {
	dsn := getenv("POSTGRES_DSN", "postgres://localhost:5432/apa?sslmode=disable")
	addr := getenv("WS_ADDR", ":8080")
	initial := int64(100000)

	st, err := store.New(dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := st.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Optional seed from env
	seedAgent(st, "AGENT1_NAME", "AGENT1_KEY", initial)
	seedAgent(st, "AGENT2_NAME", "AGENT2_KEY", initial)

	led := ledger.New(st)
	if err := st.EnsureDefaultRooms(context.Background()); err != nil {
		log.Fatal(err)
	}
	srv := ws.NewServer(st, led)

	h := http.NewServeMux()
	h.HandleFunc("/ws", srv.HandleWS)
	h.HandleFunc("/healthz", healthHandler(st))
	h.HandleFunc("/api/agents", withAdminAuth(agentsHandler(st)))
	h.HandleFunc("/api/accounts", withAdminAuth(accountsHandler(st)))
	h.HandleFunc("/api/ledger", withAdminAuth(ledgerHandler(st)))
	h.HandleFunc("/api/topup", withAdminAuth(topupHandler(st)))
	h.HandleFunc("/api/leaderboard", withAdminAuth(leaderboardHandler(st)))
	h.HandleFunc("/api/rooms", withAdminAuth(roomsHandler(st)))

	staticDir := filepath.Join("internal", "ws", "static")
	h.Handle("/", http.FileServer(http.Dir(staticDir)))

	server := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	log.Println("ws listening", addr)
	log.Fatal(server.ListenAndServe())
}

func seedAgent(st *store.Store, nameEnv, keyEnv string, initial int64) {
	name := os.Getenv(nameEnv)
	key := os.Getenv(keyEnv)
	if name == "" || key == "" {
		return
	}
	ctx := context.Background()
	agent, err := st.GetAgentByAPIKey(ctx, key)
	if err == nil && agent != nil {
		_ = st.EnsureAccount(ctx, agent.ID, initial)
		return
	}
	id, err := st.CreateAgent(ctx, name, key)
	if err != nil {
		log.Println("seed agent error", err)
		return
	}
	_ = st.EnsureAccount(ctx, id, initial)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func healthHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := st.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "db": "down"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "db": "up"})
	}
}

func agentsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		items, err := st.ListAgents(r.Context(), limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  items,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func accountsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent_id")
		limit, offset := parsePagination(r)
		items, err := st.ListAccounts(r.Context(), agentID, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  items,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func ledgerHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		f := store.LedgerFilter{
			AgentID: r.URL.Query().Get("agent_id"),
			HandID:  r.URL.Query().Get("hand_id"),
		}
		if v := r.URL.Query().Get("from"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				f.From = &t
			}
		}
		if v := r.URL.Query().Get("to"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				f.To = &t
			}
		}
		items, err := st.ListLedgerEntries(r.Context(), f, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  items,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func topupHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			AgentID  string `json:"agent_id"`
			AmountCC int64  `json:"amount_cc"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.AgentID == "" || body.AmountCC <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		refID := strconv.FormatInt(time.Now().UnixNano(), 10)
		bal, err := st.Credit(r.Context(), body.AgentID, body.AmountCC, "topup_credit", "topup", refID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "balance_cc": bal})
	}
}

func roomsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := st.ListRooms(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case http.MethodPost:
			var body struct {
				Name       string `json:"name"`
				MinBuyinCC int64  `json:"min_buyin_cc"`
				SmallBlind int64  `json:"small_blind_cc"`
				BigBlind   int64  `json:"big_blind_cc"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Name == "" || body.MinBuyinCC <= 0 || body.SmallBlind <= 0 || body.BigBlind <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id, err := st.CreateRoom(r.Context(), body.Name, body.MinBuyinCC, body.SmallBlind, body.BigBlind)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "room_id": id})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

type leaderboardCache struct {
	mu      sync.Mutex
	expires time.Time
	data    []store.LeaderboardEntry
}

func leaderboardHandler(st *store.Store) http.HandlerFunc {
	cache := &leaderboardCache{}
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		cache.mu.Lock()
		if time.Now().Before(cache.expires) && offset == 0 {
			data := cache.data
			cache.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":  data,
				"limit":  limit,
				"offset": offset,
			})
			return
		}
		cache.mu.Unlock()

		items, err := st.ListLeaderboard(r.Context(), limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if offset == 0 {
			cache.mu.Lock()
			cache.data = items
			cache.expires = time.Now().Add(60 * time.Second)
			cache.mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  items,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func withAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey != "" {
			if !checkAdminAuth(r, adminKey) {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func checkAdminAuth(r *http.Request, adminKey string) bool {
	if v := r.Header.Get("X-Admin-Key"); v == adminKey {
		return true
	}
	auth := r.Header.Get("Authorization")
	prefix := "Bearer "
	if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
		return auth[len(prefix):] == adminKey
	}
	return false
}

func parsePagination(r *http.Request) (int, int) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
