package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"silicon-casino/internal/ledger"
	"silicon-casino/internal/logging"
	"silicon-casino/internal/store"
	"silicon-casino/internal/ws"

	"github.com/rs/zerolog/log"
)

func main() {
	logging.Init()
	dsn := getenv("POSTGRES_DSN", "postgres://localhost:5432/apa?sslmode=disable")
	addr := getenv("WS_ADDR", ":8080")
	initial := int64(100000)

	st, err := store.New(dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("store init failed")
	}
	if err := st.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("db ping failed")
	}

	// Optional seed from env
	seedAgent(st, "AGENT1_NAME", "AGENT1_KEY", initial)
	seedAgent(st, "AGENT2_NAME", "AGENT2_KEY", initial)

	led := ledger.New(st)
	if err := st.EnsureDefaultRooms(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("ensure default rooms failed")
	}
	if err := st.EnsureDefaultProviderRates(context.Background(), defaultProviderRates()); err != nil {
		log.Fatal().Err(err).Msg("ensure provider rates failed")
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
	h.HandleFunc("/api/public/leaderboard", publicLeaderboardHandler(st))
	h.HandleFunc("/api/rooms", withAdminAuth(roomsHandler(st)))
	h.HandleFunc("/api/providers/rates", withAdminAuth(providerRatesHandler(st)))
	h.HandleFunc("/api/public/rooms", publicRoomsHandler(st))
	h.HandleFunc("/api/public/tables", publicTablesHandler(st))
	h.HandleFunc("/api/public/agent-table", publicAgentTableHandler(srv))
	h.HandleFunc("/api/agents/register", registerAgentHandler(st))
	h.HandleFunc("/api/agents/status", withAgentAuth(st, agentStatusHandler(st)))
	h.HandleFunc("/api/agents/me", withAgentAuth(st, agentMeHandler(st)))
	h.HandleFunc("/api/agents/claim", claimAgentHandler(st))
	h.HandleFunc("/api/agents/bind_key", withAgentAuth(st, bindKeyHandler(st)))
	staticDir := filepath.Join("internal", "ws", "static")
	h.Handle("/", http.FileServer(http.Dir(staticDir)))
	skillServer := http.FileServer(http.Dir(filepath.Join("api", "skill")))
	h.Handle("/skill.md", skillServer)
	h.Handle("/heartbeat.md", skillServer)
	h.Handle("/messaging.md", skillServer)
	h.Handle("/skill.json", skillServer)

	server := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	log.Info().Str("addr", addr).Msg("ws listening")
	log.Fatal().Err(server.ListenAndServe()).Msg("server stopped")
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
		log.Error().Err(err).Msg("seed agent error")
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

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func defaultProviderRates() []store.ProviderRate {
	ccPerUSD := getenvFloat("CC_PER_USD", 1000)
	return []store.ProviderRate{
		{
			Provider:            "openai",
			PricePer1KTokensUSD: getenvFloat("OPENAI_PRICE_PER_1K_USD", 0.0001),
			CCPerUSD:            ccPerUSD,
			Weight:              getenvFloat("OPENAI_WEIGHT", 1.0),
		},
		{
			Provider:            "kimi",
			PricePer1KTokensUSD: getenvFloat("KIMI_PRICE_PER_1K_USD", 0.0001),
			CCPerUSD:            ccPerUSD,
			Weight:              getenvFloat("KIMI_WEIGHT", 1.0),
		},
	}
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

func providerRatesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := st.ListProviderRates(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case http.MethodPost:
			var body struct {
				Provider            string  `json:"provider"`
				PricePer1KTokensUSD float64 `json:"price_per_1k_tokens_usd"`
				CCPerUSD            float64 `json:"cc_per_usd"`
				Weight              float64 `json:"weight"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body.Provider = strings.ToLower(strings.TrimSpace(body.Provider))
			if body.Provider == "" || body.PricePer1KTokensUSD <= 0 || body.CCPerUSD <= 0 || body.Weight <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := st.UpsertProviderRate(r.Context(), body.Provider, body.PricePer1KTokensUSD, body.CCPerUSD, body.Weight); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func publicRoomsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := st.ListRooms(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// public subset
		out := []map[string]any{}
		for _, it := range items {
			out = append(out, map[string]any{
				"id":             it.ID,
				"name":           it.Name,
				"min_buyin_cc":   it.MinBuyinCC,
				"small_blind_cc": it.SmallBlindCC,
				"big_blind_cc":   it.BigBlindCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
	}
}

func publicTablesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room_id")
		limit, offset := parsePagination(r)
		items, err := st.ListTables(r.Context(), roomID, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"table_id":       it.ID,
				"room_id":        it.RoomID,
				"status":         it.Status,
				"created_at":     it.CreatedAt,
				"small_blind_cc": it.SmallBlindCC,
				"big_blind_cc":   it.BigBlindCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func publicAgentTableHandler(srv *ws.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tableID, roomID, ok := srv.FindTableByAgent(agentID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": agentID,
			"room_id":  roomID,
			"table_id": tableID,
		})
	}
}

func publicLeaderboardHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parsePagination(r)
		items, err := st.ListLeaderboard(r.Context(), limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"agent_id": it.AgentID,
				"name":     it.Name,
				"net_cc":   it.NetCC,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func registerAgentHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		apiKey := "apa_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		id, err := st.CreateAgent(r.Context(), body.Name, apiKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		claimCode := "apa_claim_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if _, err := st.CreateAgentClaim(r.Context(), id, claimCode); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = st.EnsureAccount(r.Context(), id, 10000)
		claimURL := "https://apa.network/claim/" + claimCode
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent": map[string]any{
				"agent_id":          id,
				"api_key":           apiKey,
				"claim_url":         claimURL,
				"verification_code": claimCode,
			},
		})
	}
}

func agentStatusHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := r.Context().Value(agentContextKey{}).(*store.Agent)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": agent.Status})
	}
}

func agentMeHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := r.Context().Value(agentContextKey{}).(*store.Agent)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id":   agent.ID,
			"name":       agent.Name,
			"status":     agent.Status,
			"created_at": agent.CreatedAt,
		})
	}
}

func claimAgentHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			AgentID   string `json:"agent_id"`
			ClaimCode string `json:"claim_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		claim, err := st.GetAgentClaimByAgent(r.Context(), body.AgentID)
		if err != nil || claim.ClaimCode != body.ClaimCode {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := st.MarkAgentClaimed(r.Context(), body.AgentID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func bindKeyHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Provider  string  `json:"provider"`
			APIKey    string  `json:"api_key"`
			BudgetUSD float64 `json:"budget_usd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		agent := r.Context().Value(agentContextKey{}).(*store.Agent)
		body.Provider = strings.ToLower(strings.TrimSpace(body.Provider))
		if body.Provider == "" || body.APIKey == "" || body.BudgetUSD <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_request"})
			return
		}
		if maxBudget := getenvFloat("MAX_BUDGET_USD", 20); body.BudgetUSD > maxBudget {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "budget_exceeds_limit"})
			return
		}
		if body.Provider != "openai" && body.Provider != "kimi" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_provider"})
			return
		}

		if blocked, reason, err := st.IsAgentBlacklisted(r.Context(), agent.ID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if blocked {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "agent_blacklisted", "reason": reason})
			return
		}

		if last, err := st.LastSuccessfulKeyBindAt(r.Context(), agent.ID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if last != nil {
			cooldown := time.Duration(getenvFloat("BIND_KEY_COOLDOWN_MINUTES", 60)) * time.Minute
			if time.Since(*last) < cooldown {
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "cooldown_active"})
				return
			}
		}

		keyHash := store.HashAPIKey(body.APIKey)
		if existing, err := st.GetAgentKeyByHash(r.Context(), keyHash); err == nil && existing != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "api_key_already_bound"})
			return
		} else if err != nil && err != store.ErrNotFound {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := verifyVendorKey(r.Context(), body.Provider, body.APIKey); err != nil {
			_ = st.RecordAgentKeyAttempt(r.Context(), agent.ID, body.Provider, "invalid_key")
			if n, err := st.CountConsecutiveInvalidKeyAttempts(r.Context(), agent.ID); err == nil && n >= 3 {
				_ = st.BlacklistAgent(r.Context(), agent.ID, "too_many_invalid_keys")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "agent_blacklisted"})
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_vendor_key"})
			return
		}

		rate, err := st.GetProviderRate(r.Context(), body.Provider)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_provider"})
			return
		}
		credit := store.ComputeCCFromBudgetUSD(body.BudgetUSD, rate.CCPerUSD, rate.Weight)
		if credit <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_request"})
			return
		}

		keyID, err := st.CreateAgentKey(r.Context(), agent.ID, body.Provider, keyHash)
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "api_key_already_bound"})
			return
		}
		_ = st.RecordAgentKeyAttempt(r.Context(), agent.ID, body.Provider, "success")
		_ = st.EnsureAccount(r.Context(), agent.ID, 0)
		newBal, err := st.Credit(r.Context(), agent.ID, credit, "key_credit", "agent_key", keyID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         true,
			"added_cc":   credit,
			"balance_cc": newBal,
		})
	}
}

type agentContextKey struct{}

func withAgentAuth(st *store.Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		prefix := "Bearer "
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		apiKey := auth[len(prefix):]
		agent, err := st.GetAgentByAPIKey(r.Context(), apiKey)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), agentContextKey{}, agent)
		next(w, r.WithContext(ctx))
	}
}

func verifyVendorKey(ctx context.Context, provider, apiKey string) error {
	base := getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	if provider == "kimi" {
		base = getenv("KIMI_BASE_URL", "https://api.moonshot.ai/v1")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("invalid_vendor_key")
	}
	return nil
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
