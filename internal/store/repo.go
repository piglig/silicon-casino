package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrNotFound = errors.New("not found")

// Store wraps DB access.
type Store struct {
	DB *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (s *Store) GetAgentByAPIKey(ctx context.Context, apiKey string) (*Agent, error) {
	hash := HashAPIKey(apiKey)
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, api_key_hash, status, created_at FROM agents WHERE api_key_hash = $1`, hash)
	var a Agent
	if err := row.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.Status, &a.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetAccountBalance(ctx context.Context, agentID string) (int64, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT balance_cc FROM accounts WHERE agent_id = $1`, agentID)
	var bal int64
	if err := row.Scan(&bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return bal, nil
}

func (s *Store) CreateTable(ctx context.Context, roomID, status string, sb, bb int64) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO tables (id, room_id, status, small_blind_cc, big_blind_cc) VALUES ($1,$2,$3,$4,$5)`, id, roomID, status, sb, bb)
	return id, err
}

func (s *Store) CreateHand(ctx context.Context, tableID string) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO hands (id, table_id) VALUES ($1,$2)`, id, tableID)
	return id, err
}

func (s *Store) EndHand(ctx context.Context, handID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE hands SET ended_at = now() WHERE id = $1`, handID)
	return err
}

func (s *Store) RecordAction(ctx context.Context, handID, agentID, actionType string, amount int64) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO actions (hand_id, agent_id, action_type, amount_cc) VALUES ($1,$2,$3,$4)`, handID, agentID, actionType, amount)
	return err
}

func (s *Store) RecordLedgerEntry(ctx context.Context, tx *sql.Tx, agentID, entryType string, amount int64, refType, refID string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO ledger_entries (id, agent_id, type, amount_cc, ref_type, ref_id) VALUES ($1,$2,$3,$4,$5,$6)`, NewID(), agentID, entryType, amount, refType, refID)
	return err
}

func (s *Store) RecordProxyCall(ctx context.Context, agentID, model, provider string, prompt, completion, total int, cost int64) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO proxy_calls (id, agent_id, prompt_tokens, completion_tokens, total_tokens, model, provider, cost_cc) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, agentID, prompt, completion, total, model, provider, cost)
	return id, err
}

func (s *Store) Debit(ctx context.Context, agentID string, amount int64, entryType, refType, refID string) (int64, error) {
	if amount < 0 {
		return 0, errors.New("amount must be positive")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var bal int64
	row := tx.QueryRowContext(ctx, `SELECT balance_cc FROM accounts WHERE agent_id = $1 FOR UPDATE`, agentID)
	if err := row.Scan(&bal); err != nil {
		return 0, err
	}
	if bal < amount {
		return 0, errors.New("insufficient_balance")
	}
	newBal := bal - amount
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance_cc = $1, updated_at = now() WHERE agent_id = $2`, newBal, agentID)
	if err != nil {
		return 0, err
	}
	if err := s.RecordLedgerEntry(ctx, tx, agentID, entryType, -amount, refType, refID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newBal, nil
}

func (s *Store) Credit(ctx context.Context, agentID string, amount int64, entryType, refType, refID string) (int64, error) {
	if amount < 0 {
		return 0, errors.New("amount must be positive")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var bal int64
	row := tx.QueryRowContext(ctx, `SELECT balance_cc FROM accounts WHERE agent_id = $1 FOR UPDATE`, agentID)
	if err := row.Scan(&bal); err != nil {
		return 0, err
	}
	newBal := bal + amount
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance_cc = $1, updated_at = now() WHERE agent_id = $2`, newBal, agentID)
	if err != nil {
		return 0, err
	}
	if err := s.RecordLedgerEntry(ctx, tx, agentID, entryType, amount, refType, refID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newBal, nil
}

func (s *Store) EnsureAccount(ctx context.Context, agentID string, initial int64) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO accounts (agent_id, balance_cc) VALUES ($1,$2) ON CONFLICT (agent_id) DO NOTHING`, agentID, initial)
	return err
}

func (s *Store) CreateAgent(ctx context.Context, name, apiKey string) (string, error) {
	id := NewID()
	hash := HashAPIKey(apiKey)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO agents (id, name, api_key_hash, status) VALUES ($1,$2,$3,'pending')`, id, name, hash)
	return id, err
}

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.DB.PingContext(ctx)
}

func (s *Store) ListRooms(ctx context.Context) ([]Room, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status, created_at FROM rooms WHERE status = 'active' ORDER BY min_buyin_cc ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Room{}
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.MinBuyinCC, &r.SmallBlindCC, &r.BigBlindCC, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) GetRoom(ctx context.Context, id string) (*Room, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status, created_at FROM rooms WHERE id = $1`, id)
	var r Room
	if err := row.Scan(&r.ID, &r.Name, &r.MinBuyinCC, &r.SmallBlindCC, &r.BigBlindCC, &r.Status, &r.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateRoom(ctx context.Context, name string, minBuyin, sb, bb int64) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO rooms (id, name, min_buyin_cc, small_blind_cc, big_blind_cc, status) VALUES ($1,$2,$3,$4,$5,'active')`, id, name, minBuyin, sb, bb)
	return id, err
}

func (s *Store) CountRooms(ctx context.Context) (int, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM rooms`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (s *Store) EnsureDefaultRooms(ctx context.Context) error {
	c, err := s.CountRooms(ctx)
	if err != nil {
		return err
	}
	if c > 0 {
		return nil
	}
	_, err = s.CreateRoom(ctx, "Low", 1000, 50, 100)
	if err != nil {
		return err
	}
	_, err = s.CreateRoom(ctx, "Mid", 5000, 100, 200)
	if err != nil {
		return err
	}
	_, err = s.CreateRoom(ctx, "High", 20000, 500, 1000)
	return err
}

type LedgerFilter struct {
	AgentID string
	HandID  string
	From    *time.Time
	To      *time.Time
}

func (s *Store) ListAgents(ctx context.Context, limit, offset int) ([]Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, api_key_hash, status, created_at FROM agents ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) CreateAgentClaim(ctx context.Context, agentID, claimCode string) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO agent_claims (id, agent_id, claim_code, status) VALUES ($1,$2,$3,'pending')`, id, agentID, claimCode)
	return id, err
}

func (s *Store) GetAgentClaimByAgent(ctx context.Context, agentID string) (*AgentClaim, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, agent_id, claim_code, status, created_at FROM agent_claims WHERE agent_id = $1`, agentID)
	var c AgentClaim
	if err := row.Scan(&c.ID, &c.AgentID, &c.ClaimCode, &c.Status, &c.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) MarkAgentClaimed(ctx context.Context, agentID string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE agents SET status = 'claimed' WHERE id = $1`, agentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE agent_claims SET status = 'claimed' WHERE agent_id = $1`, agentID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateAgentKey(ctx context.Context, agentID, provider, apiKeyHash string) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO agent_keys (id, agent_id, provider, api_key_hash, status) VALUES ($1,$2,$3,$4,'active')`, id, agentID, provider, apiKeyHash)
	return id, err
}

func (s *Store) GetAgentKeyByHash(ctx context.Context, apiKeyHash string) (*AgentKey, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, agent_id, provider, api_key_hash, status, created_at FROM agent_keys WHERE api_key_hash = $1`, apiKeyHash)
	var k AgentKey
	if err := row.Scan(&k.ID, &k.AgentID, &k.Provider, &k.APIKeyHash, &k.Status, &k.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (s *Store) ListProviderRates(ctx context.Context) ([]ProviderRate, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT provider, price_per_1k_tokens_usd, cc_per_usd, weight, updated_at FROM provider_rates ORDER BY provider ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProviderRate{}
	for rows.Next() {
		var r ProviderRate
		if err := rows.Scan(&r.Provider, &r.PricePer1KTokensUSD, &r.CCPerUSD, &r.Weight, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) GetProviderRate(ctx context.Context, provider string) (*ProviderRate, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT provider, price_per_1k_tokens_usd, cc_per_usd, weight, updated_at FROM provider_rates WHERE provider = $1`, provider)
	var r ProviderRate
	if err := row.Scan(&r.Provider, &r.PricePer1KTokensUSD, &r.CCPerUSD, &r.Weight, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) UpsertProviderRate(ctx context.Context, provider string, pricePer1KTokensUSD, ccPerUSD, weight float64) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO provider_rates (provider, price_per_1k_tokens_usd, cc_per_usd, weight)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (provider) DO UPDATE
		SET price_per_1k_tokens_usd = EXCLUDED.price_per_1k_tokens_usd,
		    cc_per_usd = EXCLUDED.cc_per_usd,
		    weight = EXCLUDED.weight,
		    updated_at = now()
	`, provider, pricePer1KTokensUSD, ccPerUSD, weight)
	return err
}

func (s *Store) EnsureDefaultProviderRates(ctx context.Context, defaults []ProviderRate) error {
	for _, r := range defaults {
		_, err := s.DB.ExecContext(ctx, `
			INSERT INTO provider_rates (provider, price_per_1k_tokens_usd, cc_per_usd, weight)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (provider) DO NOTHING
		`, r.Provider, r.PricePer1KTokensUSD, r.CCPerUSD, r.Weight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAccounts(ctx context.Context, agentID string, limit, offset int) ([]Account, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if agentID == "" {
		rows, err = s.DB.QueryContext(ctx, `SELECT agent_id, balance_cc, updated_at FROM accounts ORDER BY updated_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	} else {
		rows, err = s.DB.QueryContext(ctx, `SELECT agent_id, balance_cc, updated_at FROM accounts WHERE agent_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`, agentID, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.AgentID, &a.BalanceCC, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) ListLedgerEntries(ctx context.Context, f LedgerFilter, limit, offset int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	where := "WHERE 1=1"
	args := []any{}
	if f.AgentID != "" {
		args = append(args, f.AgentID)
		where += fmt.Sprintf(" AND agent_id = $%d", len(args))
	}
	if f.HandID != "" {
		args = append(args, f.HandID)
		where += fmt.Sprintf(" AND ref_type = 'hand' AND ref_id = $%d", len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}
	args = append(args, limit, offset)
	q := `SELECT id, agent_id, type, amount_cc, ref_type, ref_id, created_at FROM ledger_entries ` + where + ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)-1) + ` OFFSET $` + fmt.Sprintf("%d", len(args))
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.AgentID, &e.Type, &e.AmountCC, &e.RefType, &e.RefID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) ListLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, a.name, COALESCE(SUM(l.amount_cc), 0) AS net_cc
		FROM agents a
		LEFT JOIN ledger_entries l ON l.agent_id = a.id
		GROUP BY a.id, a.name
		ORDER BY net_cc DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LeaderboardEntry{}
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.AgentID, &e.Name, &e.NetCC); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
