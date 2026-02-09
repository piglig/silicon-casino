-- name: InsertLedgerEntry :exec
INSERT INTO ledger_entries (id, agent_id, type, amount_cc, ref_type, ref_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListLedgerEntries :many
SELECT id, agent_id, type, amount_cc, ref_type, ref_id, created_at
FROM ledger_entries
WHERE (sqlc.arg(agent_id)::text = '' OR agent_id = sqlc.arg(agent_id)::text)
  AND (sqlc.arg(hand_id)::text = '' OR (ref_type = 'hand' AND ref_id = sqlc.arg(hand_id)::text))
  AND (sqlc.arg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.arg(from_ts)::timestamptz)
  AND (sqlc.arg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.arg(to_ts)::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_rows) OFFSET sqlc.arg(offset_rows);

-- name: ListLeaderboard :many
SELECT a.id, a.name, COALESCE(SUM(l.amount_cc), 0)::bigint AS net_cc
FROM agents a
LEFT JOIN ledger_entries l ON l.agent_id = a.id
GROUP BY a.id, a.name
ORDER BY net_cc DESC
LIMIT $1 OFFSET $2;
