package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"silicon-casino/internal/store"
)

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
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
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
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
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
		var body struct {
			AgentID  string `json:"agent_id"`
			AmountCC int64  `json:"amount_cc"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.AgentID == "" || body.AmountCC <= 0 {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		refID := strconv.FormatInt(time.Now().UnixNano(), 10)
		bal, err := st.Credit(r.Context(), body.AgentID, body.AmountCC, "topup_credit", "topup", refID)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
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
				writeHTTPError(w, http.StatusInternalServerError, "internal_error")
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
				writeHTTPError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			if body.Name == "" || body.MinBuyinCC <= 0 || body.SmallBlind <= 0 || body.BigBlind <= 0 {
				writeHTTPError(w, http.StatusBadRequest, "invalid_request")
				return
			}
			id, err := st.CreateRoom(r.Context(), body.Name, body.MinBuyinCC, body.SmallBlind, body.BigBlind)
			if err != nil {
				writeHTTPError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "room_id": id})
		default:
			writeHTTPError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	}
}

func providerRatesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := st.ListProviderRates(r.Context())
			if err != nil {
				writeHTTPError(w, http.StatusInternalServerError, "internal_error")
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
				writeHTTPError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			body.Provider = strings.ToLower(strings.TrimSpace(body.Provider))
			if body.Provider == "" || body.PricePer1KTokensUSD <= 0 || body.CCPerUSD <= 0 || body.Weight <= 0 {
				writeHTTPError(w, http.StatusBadRequest, "invalid_request")
				return
			}
			if err := st.UpsertProviderRate(r.Context(), body.Provider, body.PricePer1KTokensUSD, body.CCPerUSD, body.Weight); err != nil {
				writeHTTPError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			writeHTTPError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	}
}
