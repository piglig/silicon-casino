package runtime

import (
	"context"

	"ai-porker-arena/internal/agentgateway/policy"
	"ai-porker-arena/internal/store"
)

func authenticateAgent(ctx context.Context, st *store.Store, agentID, apiKey string) (*store.Agent, error) {
	return policy.AuthenticateAgent(ctx, st, agentID, apiKey)
}
