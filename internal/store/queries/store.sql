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

-- name: EnsureAccount :exec
INSERT INTO accounts (agent_id, balance_cc)
VALUES ($1, $2)
ON CONFLICT (agent_id) DO NOTHING;

-- name: GetAccountBalance :one
SELECT balance_cc
FROM accounts
WHERE agent_id = $1;

-- name: GetAccountBalanceForUpdate :one
SELECT balance_cc
FROM accounts
WHERE agent_id = $1
FOR UPDATE;

-- name: UpdateAccountBalance :exec
UPDATE accounts
SET balance_cc = $1, updated_at = now()
WHERE agent_id = $2;

-- name: ListAccounts :many
SELECT agent_id, balance_cc, updated_at
FROM accounts
WHERE ($1::text = '' OR agent_id = $1)
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateRoom :exec
INSERT INTO rooms (id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status)
VALUES ($1, $2, $3, $4, $5, 'active');

-- name: GetRoom :one
SELECT id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status, created_at
FROM rooms
WHERE id = $1;

-- name: ListRooms :many
SELECT id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status, created_at
FROM rooms
WHERE status = 'active'
ORDER BY min_buyin_cc ASC;

-- name: CountRooms :one
SELECT COUNT(1)::int
FROM rooms;

-- name: CreateTable :exec
INSERT INTO tables (id, room_id, status, small_blind_cc, big_blind_cc)
VALUES ($1, $2, $3, $4, $5);

-- name: ListTables :many
SELECT id, room_id, status, small_blind_cc, big_blind_cc, created_at
FROM tables
WHERE status = 'active'
  AND ($1::text = '' OR room_id = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateHand :exec
INSERT INTO hands (id, table_id)
VALUES ($1, $2);

-- name: EndHand :exec
UPDATE hands
SET ended_at = now()
WHERE id = $1;

-- name: RecordAction :exec
INSERT INTO actions (id, hand_id, agent_id, action_type, amount_cc)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertLedgerEntry :exec
INSERT INTO ledger_entries (id, agent_id, type, amount_cc, ref_type, ref_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListLedgerEntries :many
SELECT id, agent_id, type, amount_cc, ref_type, ref_id, created_at
FROM ledger_entries
WHERE ($1::text = '' OR agent_id = $1)
  AND ($2::text = '' OR (ref_type = 'hand' AND ref_id = $2))
  AND ($3::timestamptz IS NULL OR created_at >= $3)
  AND ($4::timestamptz IS NULL OR created_at <= $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: RecordProxyCall :exec
INSERT INTO proxy_calls (id, agent_id, prompt_tokens, completion_tokens, total_tokens, model, provider, cost_cc)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: CreateAgentClaim :exec
INSERT INTO agent_claims (id, agent_id, claim_code, status)
VALUES ($1, $2, $3, 'pending');

-- name: GetAgentClaimByAgent :one
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

-- name: GetAgentKeyByHash :one
SELECT id, agent_id, provider, api_key_hash, status, created_at
FROM agent_keys
WHERE api_key_hash = $1;

-- name: ListProviderRates :many
SELECT provider, price_per_1k_tokens_usd, cc_per_usd, weight, updated_at
FROM provider_rates
ORDER BY provider ASC;

-- name: GetProviderRate :one
SELECT provider, price_per_1k_tokens_usd, cc_per_usd, weight, updated_at
FROM provider_rates
WHERE provider = $1;

-- name: UpsertProviderRate :exec
INSERT INTO provider_rates (provider, price_per_1k_tokens_usd, cc_per_usd, weight)
VALUES ($1, $2, $3, $4)
ON CONFLICT (provider) DO UPDATE
SET price_per_1k_tokens_usd = EXCLUDED.price_per_1k_tokens_usd,
    cc_per_usd = EXCLUDED.cc_per_usd,
    weight = EXCLUDED.weight,
    updated_at = now();

-- name: EnsureDefaultProviderRate :exec
INSERT INTO provider_rates (provider, price_per_1k_tokens_usd, cc_per_usd, weight)
VALUES ($1, $2, $3, $4)
ON CONFLICT (provider) DO NOTHING;

-- name: IsAgentBlacklisted :one
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

-- name: LastSuccessfulKeyBindAt :one
SELECT created_at
FROM agent_key_attempts
WHERE agent_id = $1 AND status = 'success'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListAgentKeyAttemptStatuses :many
SELECT status
FROM agent_key_attempts
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: ListLeaderboard :many
SELECT a.id, a.name, COALESCE(SUM(l.amount_cc), 0)::bigint AS net_cc
FROM agents a
LEFT JOIN ledger_entries l ON l.agent_id = a.id
GROUP BY a.id, a.name
ORDER BY net_cc DESC
LIMIT $1 OFFSET $2;
