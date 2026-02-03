package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"silicon-casino/internal/store"
)

type Handler struct {
	Store      *store.Store
	HTTPClient *http.Client
	OpenAIKey  string
	KimiKey    string
	OpenAIBase string
	KimiBase   string
}

type ChatRequest struct {
	Model    string      `json:"model"`
	Messages interface{} `json:"messages"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	Usage Usage `json:"usage"`
}

func NewHandler(st *store.Store) *Handler {
	openaiBase := getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	kimiBase := getenv("KIMI_BASE_URL", "https://api.moonshot.ai/v1")
	return &Handler{
		Store:      st,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		OpenAIKey:  os.Getenv("OPENAI_API_KEY"),
		KimiKey:    os.Getenv("KIMI_API_KEY"),
		OpenAIBase: openaiBase,
		KimiBase:   kimiBase,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	apiKey, err := extractBearer(r.Header.Get("Authorization"))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_api_key")
		return
	}
	agent, err := h.Store.GetAgentByAPIKey(r.Context(), apiKey)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid_api_key")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request")
		return
	}
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request")
		return
	}
	provider, base, key := h.route(req.Model)
	if provider == "" {
		writeErr(w, http.StatusBadRequest, "unsupported_model")
		return
	}
	if key == "" {
		writeErr(w, http.StatusBadGateway, "upstream_error")
		return
	}

	upstreamURL := strings.TrimRight(base, "/") + "/chat/completions"
	respBody, usage, err := h.forward(r.Context(), upstreamURL, key, body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "upstream_error")
		return
	}

	rate, err := h.Store.GetProviderRate(r.Context(), provider)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_provider")
		return
	}
	cost := h.computeCost(usage.TotalTokens, rate)
	if cost > 0 {
		callID, _ := h.Store.RecordProxyCall(r.Context(), agent.ID, req.Model, provider, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, cost)
		newBal, err := h.Store.Debit(r.Context(), agent.ID, cost, "proxy_debit", "proxy_call", callID)
		if err != nil {
			writeErr(w, http.StatusPaymentRequired, "insufficient_balance")
			return
		}
		_ = newBal
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (h *Handler) forward(ctx context.Context, url, key string, body []byte) ([]byte, Usage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, Usage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, Usage{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Usage{}, err
	}
	if resp.StatusCode >= 400 {
		return nil, Usage{}, errors.New("upstream_error")
	}
	var parsed ChatResponse
	_ = json.Unmarshal(b, &parsed)
	return b, parsed.Usage, nil
}

func (h *Handler) route(model string) (provider, base, key string) {
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-") {
		return "openai", h.OpenAIBase, h.OpenAIKey
	}
	if strings.HasPrefix(model, "kimi-") {
		return "kimi", h.KimiBase, h.KimiKey
	}
	return "", "", ""
}

func (h *Handler) computeCost(totalTokens int, rate *store.ProviderRate) int64 {
	return store.ComputeCCFromTokens(totalTokens, rate.PricePer1KTokensUSD, rate.CCPerUSD, rate.Weight)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func extractBearer(v string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(v, prefix) {
		return "", errors.New("missing bearer")
	}
	return strings.TrimPrefix(v, prefix), nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
