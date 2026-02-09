package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"silicon-casino/internal/config"
	"silicon-casino/internal/store"
)

type agentContextKey struct{}

func agentAuthMiddleware(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyVendorKey(ctx context.Context, cfg config.ServerConfig, provider, apiKey string) error {
	base := cfg.OpenAIBaseURL
	if provider == "kimi" {
		base = cfg.KimiBaseURL
	}
	client := &http.Client{Timeout: 10 * time.Second}
	url := strings.TrimRight(base, "/") + "/models"
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			if attempt == 0 && ctx.Err() == nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 && attempt == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("invalid_vendor_key")
		}
		return nil
	}
	return fmt.Errorf("invalid_vendor_key")
}

func adminAuthMiddleware(adminKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminKey != "" {
				if !checkAdminAuth(r, adminKey) {
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unauthorized"})
					return
				}
			}
			next.ServeHTTP(w, r)
		})
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
