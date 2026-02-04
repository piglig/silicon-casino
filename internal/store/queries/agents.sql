-- name: CreateAgent :exec
INSERT INTO agents (id, name, api_key_hash, status) VALUES ($1, $2, $3, 'pending');

-- name: GetAgentByAPIKeyHash :one
SELECT id, name, api_key_hash, status, created_at
FROM agents
WHERE api_key_hash = $1;

-- name: GetAgentByID :one
SELECT id, name, api_key_hash, status, created_at
FROM agents
WHERE id = $1;

-- name: ListAgents :many
SELECT id, name, api_key_hash, status, created_at
FROM agents
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateAgentClaim :exec
INSERT INTO agent_claims (id, agent_id, claim_code, status)
VALUES ($1, $2, $3, 'pending');

-- name: GetAgentClaimByAgentID :one
SELECT id, agent_id, claim_code, status, created_at
FROM agent_claims
WHERE agent_id = $1;

-- name: MarkAgentStatusClaimed :exec
UPDATE agents
SET status = 'claimed'
WHERE id = $1;

-- name: MarkAgentClaimClaimed :exec
UPDATE agent_claims
SET status = 'claimed'
WHERE agent_id = $1;

-- name: CreateAgentKey :exec
INSERT INTO agent_keys (id, agent_id, provider, api_key_hash, status)
VALUES ($1, $2, $3, $4, 'active');

-- name: GetAgentKeyByAPIKeyHash :one
SELECT id, agent_id, provider, api_key_hash, status, created_at
FROM agent_keys
WHERE api_key_hash = $1;

-- name: GetAgentBlacklistReasonByAgentID :one
SELECT reason
FROM agent_blacklist
WHERE agent_id = $1;

-- name: BlacklistAgent :exec
INSERT INTO agent_blacklist (agent_id, reason)
VALUES ($1, $2)
ON CONFLICT (agent_id) DO UPDATE
SET reason = EXCLUDED.reason, created_at = now();

-- name: RecordAgentKeyAttempt :exec
INSERT INTO agent_key_attempts (id, agent_id, provider, status)
VALUES ($1, $2, $3, $4);

-- name: GetLastSuccessfulKeyBindAtByAgentID :one
SELECT created_at
FROM agent_key_attempts
WHERE agent_id = $1 AND status = 'success'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListAgentKeyAttemptStatusesByAgentID :many
SELECT status
FROM agent_key_attempts
WHERE agent_id = $1
ORDER BY created_at DESC;
