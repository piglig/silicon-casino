package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"silicon-casino/internal/config"
	"silicon-casino/internal/store"

	"github.com/go-chi/chi/v5"
)

func registerAgentHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.Name == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		apiKey := "apa_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		claimCode := "apa_claim_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		id, err := st.CreateAgent(r.Context(), body.Name, apiKey, claimCode)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = st.EnsureAccount(r.Context(), id, 10000)
		claimURL := "http://localhost:8080/claim/" + claimCode
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

func agentMeHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := r.Context().Value(agentContextKey{}).(*store.Agent)
		balance, err := st.GetAccountBalance(r.Context(), agent.ID)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id":   agent.ID,
			"name":       agent.Name,
			"status":     agent.Status,
			"balance_cc": balance,
			"created_at": agent.CreatedAt,
		})
	}
}

func claimAgentHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AgentID   string `json:"agent_id"`
			ClaimCode string `json:"claim_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		claim, err := st.GetAgentClaimByAgent(r.Context(), body.AgentID)
		if err != nil || claim.ClaimCode != body.ClaimCode {
			writeHTTPError(w, http.StatusUnauthorized, "invalid_claim")
			return
		}
		if err := st.MarkAgentClaimed(r.Context(), body.AgentID); err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func claimByCodeHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claimCode := chi.URLParam(r, "claim_code")
		if claimCode == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		claim, err := st.GetAgentClaimByCode(r.Context(), claimCode)
		if err != nil {
			writeHTTPError(w, http.StatusNotFound, "claim_not_found")
			return
		}
		if claim.Status != "claimed" {
			if err := st.MarkAgentClaimed(r.Context(), claim.AgentID); err != nil {
				writeHTTPError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			claim.Status = "claimed"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"agent_id": claim.AgentID,
			"status":   claim.Status,
		})
	}
}

func bindKeyHandler(st *store.Store, cfg config.ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Provider  string  `json:"provider"`
			APIKey    string  `json:"api_key"`
			BudgetUSD float64 `json:"budget_usd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		agent := r.Context().Value(agentContextKey{}).(*store.Agent)
		body.Provider = strings.ToLower(strings.TrimSpace(body.Provider))
		if body.Provider == "" || body.APIKey == "" || body.BudgetUSD <= 0 {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		if body.BudgetUSD > cfg.MaxBudgetUSD {
			writeHTTPError(w, http.StatusBadRequest, "budget_exceeds_limit")
			return
		}
		if body.Provider != "openai" && body.Provider != "kimi" {
			writeHTTPError(w, http.StatusBadRequest, "invalid_provider")
			return
		}

		if blocked, reason, err := st.IsAgentBlacklisted(r.Context(), agent.ID); err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		} else if blocked {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "agent_blacklisted", "reason": reason})
			return
		}

		if last, err := st.LastSuccessfulKeyBindAt(r.Context(), agent.ID); err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		} else if last != nil {
			cooldown := time.Duration(cfg.BindCooldownMins) * time.Minute
			if time.Since(*last) < cooldown {
				writeHTTPError(w, http.StatusTooManyRequests, "cooldown_active")
				return
			}
		}

		keyHash := store.HashAPIKey(body.APIKey)
		if existing, err := st.GetAgentKeyByHash(r.Context(), keyHash); err == nil && existing != nil {
			writeHTTPError(w, http.StatusConflict, "api_key_already_bound")
			return
		} else if err != nil && err != store.ErrNotFound {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}

		if !cfg.AllowAnyVendorKey {
			if err := verifyVendorKey(r.Context(), cfg, body.Provider, body.APIKey); err != nil {
				_ = st.RecordAgentKeyAttempt(r.Context(), agent.ID, body.Provider, "invalid_key")
				if n, err := st.CountConsecutiveInvalidKeyAttempts(r.Context(), agent.ID); err == nil && n >= 3 {
					_ = st.BlacklistAgent(r.Context(), agent.ID, "too_many_invalid_keys")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": "agent_blacklisted"})
					return
				}
				writeHTTPError(w, http.StatusUnauthorized, "invalid_vendor_key")
				return
			}
		}

		rate, err := st.GetProviderRate(r.Context(), body.Provider)
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid_provider")
			return
		}
		credit := store.ComputeCCFromBudgetUSD(body.BudgetUSD, rate.CCPerUSD, rate.Weight)
		if credit <= 0 {
			writeHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}

		keyID, err := st.CreateAgentKey(r.Context(), agent.ID, body.Provider, keyHash)
		if err != nil {
			writeHTTPError(w, http.StatusConflict, "api_key_already_bound")
			return
		}
		_ = st.RecordAgentKeyAttempt(r.Context(), agent.ID, body.Provider, "success")
		_ = st.EnsureAccount(r.Context(), agent.ID, 0)
		newBal, err := st.Credit(r.Context(), agent.ID, credit, "key_credit", "agent_key", keyID)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         true,
			"added_cc":   credit,
			"balance_cc": newBal,
		})
	}
}
