package store

import (
	"errors"
	"testing"
)

func TestClaimsKeysRates(t *testing.T) {
	st, ctx, cleanup := openStore(t)
	defer cleanup()

	agentID := mustCreateAgent(t, st, ctx, "A", "key-a", 0)
	claimID, err := st.CreateAgentClaim(ctx, agentID, "claim-1")
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if claimID == "" {
		t.Fatalf("claim id should not be empty")
	}
	claim, err := st.GetAgentClaimByAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if claim.ClaimCode != "claim-1" {
		t.Fatalf("unexpected claim code: %s", claim.ClaimCode)
	}
	if err := st.MarkAgentClaimed(ctx, agentID); err != nil {
		t.Fatalf("mark claimed: %v", err)
	}
	agent, err := st.GetAgentByAPIKey(ctx, "key-a")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if agent.Status != "claimed" {
		t.Fatalf("expected claimed, got %s", agent.Status)
	}

	_, err = st.CreateAgentKey(ctx, agentID, "openai", "hash-1")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	_, err = st.CreateAgentKey(ctx, agentID, "openai", "hash-1")
	if err == nil {
		t.Fatalf("expected duplicate key error")
	}

	if err := st.UpsertProviderRate(ctx, "openai", 0.001, 1000, 1.2); err != nil {
		t.Fatalf("upsert provider rate: %v", err)
	}
	rate, err := st.GetProviderRate(ctx, "openai")
	if err != nil {
		t.Fatalf("get provider rate: %v", err)
	}
	if rate.Weight != 1.2 {
		t.Fatalf("expected weight 1.2, got %v", rate.Weight)
	}

	_, err = st.GetProviderRate(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
