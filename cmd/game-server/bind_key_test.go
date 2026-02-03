package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"silicon-casino/internal/store"
	"silicon-casino/internal/testutil"
)

func TestBindKeyHandler_Success(t *testing.T) {
	st, cleanup := testutil.OpenTestStore(t)
	defer cleanup()

	ctx := context.Background()
	if err := st.UpsertProviderRate(ctx, "openai", 0.0001, 1000, 1); err != nil {
		t.Fatalf("upsert provider rate: %v", err)
	}

	apiKey := "apa_key_a"
	agentID, err := st.CreateAgent(ctx, "AgentA", apiKey)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := st.EnsureAccount(ctx, agentID, 0); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	body := []byte(`{"provider":"openai","api_key":"sk-test","budget_usd":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/bind_key", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	bindKeyHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		AddedCC   int64 `json:"added_cc"`
		BalanceCC int64 `json:"balance_cc"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AddedCC != 10000 {
		t.Fatalf("expected added_cc 10000, got %d", resp.AddedCC)
	}
	if resp.BalanceCC != 10000 {
		t.Fatalf("expected balance_cc 10000, got %d", resp.BalanceCC)
	}

	hash := store.HashAPIKey("sk-test")
	key, err := st.GetAgentKeyByHash(ctx, hash)
	if err != nil {
		t.Fatalf("get agent key: %v", err)
	}
	if key.AgentID != agentID {
		t.Fatalf("expected agent_id %s, got %s", agentID, key.AgentID)
	}
}

func TestBindKeyHandler_DuplicateKey(t *testing.T) {
	st, cleanup := testutil.OpenTestStore(t)
	defer cleanup()

	ctx := context.Background()
	if err := st.UpsertProviderRate(ctx, "openai", 0.0001, 1000, 1); err != nil {
		t.Fatalf("upsert provider rate: %v", err)
	}

	apiKey1 := "apa_key_a"
	agentID1, err := st.CreateAgent(ctx, "AgentA", apiKey1)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := st.EnsureAccount(ctx, agentID1, 0); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	body := []byte(`{"provider":"openai","api_key":"sk-test","budget_usd":10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/bind_key", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey1)
	rr := httptest.NewRecorder()
	bindKeyHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected first bind 200, got %d", rr.Code)
	}

	apiKey2 := "apa_key_b"
	agentID2, err := st.CreateAgent(ctx, "AgentB", apiKey2)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := st.EnsureAccount(ctx, agentID2, 0); err != nil {
		t.Fatalf("ensure account: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/agents/bind_key", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+apiKey2)
	rr2 := httptest.NewRecorder()
	bindKeyHandler(st).ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}
