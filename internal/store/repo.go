package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"silicon-casino/internal/store/sqlcgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// Store wraps DB access.
type Store struct {
	Pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func New(dsn string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &Store{Pool: pool, q: sqlcgen.New(pool)}, nil
}

func (s *Store) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}

func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (s *Store) GetAgentByAPIKey(ctx context.Context, apiKey string) (*Agent, error) {
	hash := HashAPIKey(apiKey)
	row, err := s.q.GetAgentByAPIKeyHash(ctx, hash)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &Agent{
		ID:         row.ID,
		Name:       row.Name,
		APIKeyHash: row.ApiKeyHash,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt.Time,
	}, nil
}

func (s *Store) GetAccountBalance(ctx context.Context, agentID string) (int64, error) {
	bal, err := s.q.GetAccountBalance(ctx, agentID)
	if err != nil {
		return 0, mapNotFound(err)
	}
	return bal, nil
}

func (s *Store) CreateTable(ctx context.Context, roomID, status string, sb, bb int64) (string, error) {
	id := NewID()
	err := s.q.CreateTable(ctx, sqlcgen.CreateTableParams{
		ID:           id,
		RoomID:       textParam(roomID),
		Status:       status,
		SmallBlindCc: sb,
		BigBlindCc:   bb,
	})
	return id, err
}

func (s *Store) ListTables(ctx context.Context, roomID string, limit, offset int) ([]Table, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListTables(ctx, sqlcgen.ListTablesParams{
		Column1: roomID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Table, 0, len(rows))
	for _, r := range rows {
		out = append(out, Table{
			ID:           r.ID,
			RoomID:       textVal(r.RoomID),
			Status:       r.Status,
			SmallBlindCC: r.SmallBlindCc,
			BigBlindCC:   r.BigBlindCc,
			CreatedAt:    r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) CreateHand(ctx context.Context, tableID string) (string, error) {
	id := NewID()
	err := s.q.CreateHand(ctx, sqlcgen.CreateHandParams{ID: id, TableID: tableID})
	return id, err
}

func (s *Store) EndHand(ctx context.Context, handID string) error {
	return s.q.EndHand(ctx, handID)
}

func (s *Store) RecordAction(ctx context.Context, handID, agentID, actionType string, amount int64) error {
	return s.q.RecordAction(ctx, sqlcgen.RecordActionParams{
		ID:         NewID(),
		HandID:     handID,
		AgentID:    agentID,
		ActionType: actionType,
		AmountCc:   amount,
	})
}

func (s *Store) RecordProxyCall(ctx context.Context, agentID, model, provider string, prompt, completion, total int, cost int64) (string, error) {
	id := NewID()
	err := s.q.RecordProxyCall(ctx, sqlcgen.RecordProxyCallParams{
		ID:               id,
		AgentID:          agentID,
		PromptTokens:     int32(prompt),
		CompletionTokens: int32(completion),
		TotalTokens:      int4Param(int32(total)),
		Model:            textParam(model),
		Provider:         textParam(provider),
		CostCc:           cost,
	})
	return id, err
}

func (s *Store) Debit(ctx context.Context, agentID string, amount int64, entryType, refType, refID string) (int64, error) {
	if amount < 0 {
		return 0, errors.New("amount must be positive")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)
	bal, err := qtx.GetAccountBalanceForUpdate(ctx, agentID)
	if err != nil {
		return 0, mapNotFound(err)
	}
	if bal < amount {
		return 0, errors.New("insufficient_balance")
	}
	newBal := bal - amount
	if err := qtx.UpdateAccountBalance(ctx, sqlcgen.UpdateAccountBalanceParams{
		BalanceCc: newBal,
		AgentID:   agentID,
	}); err != nil {
		return 0, err
	}
	if err := qtx.InsertLedgerEntry(ctx, sqlcgen.InsertLedgerEntryParams{
		ID:       NewID(),
		AgentID:  agentID,
		Type:     entryType,
		AmountCc: -amount,
		RefType:  refType,
		RefID:    refID,
	}); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newBal, nil
}

func (s *Store) Credit(ctx context.Context, agentID string, amount int64, entryType, refType, refID string) (int64, error) {
	if amount < 0 {
		return 0, errors.New("amount must be positive")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)
	bal, err := qtx.GetAccountBalanceForUpdate(ctx, agentID)
	if err != nil {
		return 0, mapNotFound(err)
	}
	newBal := bal + amount
	if err := qtx.UpdateAccountBalance(ctx, sqlcgen.UpdateAccountBalanceParams{
		BalanceCc: newBal,
		AgentID:   agentID,
	}); err != nil {
		return 0, err
	}
	if err := qtx.InsertLedgerEntry(ctx, sqlcgen.InsertLedgerEntryParams{
		ID:       NewID(),
		AgentID:  agentID,
		Type:     entryType,
		AmountCc: amount,
		RefType:  refType,
		RefID:    refID,
	}); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newBal, nil
}

func (s *Store) EnsureAccount(ctx context.Context, agentID string, initial int64) error {
	return s.q.EnsureAccount(ctx, sqlcgen.EnsureAccountParams{
		AgentID:   agentID,
		BalanceCc: initial,
	})
}

func (s *Store) CreateAgent(ctx context.Context, name, apiKey string) (string, error) {
	id := NewID()
	hash := HashAPIKey(apiKey)
	err := s.q.CreateAgent(ctx, sqlcgen.CreateAgentParams{
		ID:         id,
		Name:       name,
		ApiKeyHash: hash,
	})
	return id, err
}

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.Pool.Ping(ctx)
}

func (s *Store) ListRooms(ctx context.Context) ([]Room, error) {
	rows, err := s.q.ListRooms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Room, 0, len(rows))
	for _, r := range rows {
		out = append(out, Room{
			ID:           r.ID,
			Name:         r.Name,
			MinBuyinCC:   r.MinBuyinCc,
			SmallBlindCC: r.SmallBlindCc,
			BigBlindCC:   r.BigBlindCc,
			Status:       r.Status,
			CreatedAt:    r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) GetRoom(ctx context.Context, id string) (*Room, error) {
	r, err := s.q.GetRoom(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &Room{
		ID:           r.ID,
		Name:         r.Name,
		MinBuyinCC:   r.MinBuyinCc,
		SmallBlindCC: r.SmallBlindCc,
		BigBlindCC:   r.BigBlindCc,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt.Time,
	}, nil
}

func (s *Store) CreateRoom(ctx context.Context, name string, minBuyin, sb, bb int64) (string, error) {
	id := NewID()
	err := s.q.CreateRoom(ctx, sqlcgen.CreateRoomParams{
		ID:           id,
		Name:         name,
		MinBuyinCc:   minBuyin,
		SmallBlindCc: sb,
		BigBlindCc:   bb,
	})
	return id, err
}

func (s *Store) CountRooms(ctx context.Context) (int, error) {
	c, err := s.q.CountRooms(ctx)
	return int(c), err
}

func (s *Store) EnsureDefaultRooms(ctx context.Context) error {
	c, err := s.CountRooms(ctx)
	if err != nil {
		return err
	}
	if c > 0 {
		return nil
	}
	if _, err := s.CreateRoom(ctx, "Low", 1000, 50, 100); err != nil {
		return err
	}
	if _, err := s.CreateRoom(ctx, "Mid", 5000, 100, 200); err != nil {
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
	rows, err := s.q.ListAgents(ctx, sqlcgen.ListAgentsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Agent, 0, len(rows))
	for _, r := range rows {
		out = append(out, Agent{
			ID:         r.ID,
			Name:       r.Name,
			APIKeyHash: r.ApiKeyHash,
			Status:     r.Status,
			CreatedAt:  r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) CreateAgentClaim(ctx context.Context, agentID, claimCode string) (string, error) {
	id := NewID()
	err := s.q.CreateAgentClaim(ctx, sqlcgen.CreateAgentClaimParams{
		ID:        id,
		AgentID:   agentID,
		ClaimCode: claimCode,
	})
	return id, err
}

func (s *Store) GetAgentClaimByAgent(ctx context.Context, agentID string) (*AgentClaim, error) {
	r, err := s.q.GetAgentClaimByAgent(ctx, agentID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &AgentClaim{
		ID:        r.ID,
		AgentID:   r.AgentID,
		ClaimCode: r.ClaimCode,
		Status:    r.Status,
		CreatedAt: r.CreatedAt.Time,
	}, nil
}

func (s *Store) MarkAgentClaimed(ctx context.Context, agentID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := qtx.MarkAgentStatusClaimed(ctx, agentID); err != nil {
		return err
	}
	if err := qtx.MarkAgentClaimClaimed(ctx, agentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateAgentKey(ctx context.Context, agentID, provider, apiKeyHash string) (string, error) {
	id := NewID()
	err := s.q.CreateAgentKey(ctx, sqlcgen.CreateAgentKeyParams{
		ID:         id,
		AgentID:    agentID,
		Provider:   provider,
		ApiKeyHash: apiKeyHash,
	})
	return id, err
}

func (s *Store) GetAgentKeyByHash(ctx context.Context, apiKeyHash string) (*AgentKey, error) {
	r, err := s.q.GetAgentKeyByHash(ctx, apiKeyHash)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &AgentKey{
		ID:         r.ID,
		AgentID:    r.AgentID,
		Provider:   r.Provider,
		APIKeyHash: r.ApiKeyHash,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt.Time,
	}, nil
}

func (s *Store) ListProviderRates(ctx context.Context) ([]ProviderRate, error) {
	rows, err := s.q.ListProviderRates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderRate, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProviderRate{
			Provider:            r.Provider,
			PricePer1KTokensUSD: r.PricePer1kTokensUsd,
			CCPerUSD:            r.CcPerUsd,
			Weight:              r.Weight,
			UpdatedAt:           r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) IsAgentBlacklisted(ctx context.Context, agentID string) (bool, string, error) {
	reason, err := s.q.IsAgentBlacklisted(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, reason, nil
}

func (s *Store) BlacklistAgent(ctx context.Context, agentID, reason string) error {
	return s.q.BlacklistAgent(ctx, sqlcgen.BlacklistAgentParams{AgentID: agentID, Reason: reason})
}

func (s *Store) RecordAgentKeyAttempt(ctx context.Context, agentID, provider, status string) error {
	return s.q.RecordAgentKeyAttempt(ctx, sqlcgen.RecordAgentKeyAttemptParams{
		ID:       NewID(),
		AgentID:  agentID,
		Provider: provider,
		Status:   status,
	})
}

func (s *Store) LastSuccessfulKeyBindAt(ctx context.Context, agentID string) (*time.Time, error) {
	ts, err := s.q.LastSuccessfulKeyBindAt(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	t := ts.Time
	return &t, nil
}

func (s *Store) CountConsecutiveInvalidKeyAttempts(ctx context.Context, agentID string) (int, error) {
	statuses, err := s.q.ListAgentKeyAttemptStatuses(ctx, agentID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, status := range statuses {
		if status == "success" {
			break
		}
		if status == "invalid_key" {
			count++
		}
	}
	return count, nil
}

func (s *Store) GetProviderRate(ctx context.Context, provider string) (*ProviderRate, error) {
	r, err := s.q.GetProviderRate(ctx, provider)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &ProviderRate{
		Provider:            r.Provider,
		PricePer1KTokensUSD: r.PricePer1kTokensUsd,
		CCPerUSD:            r.CcPerUsd,
		Weight:              r.Weight,
		UpdatedAt:           r.UpdatedAt.Time,
	}, nil
}

func (s *Store) UpsertProviderRate(ctx context.Context, provider string, pricePer1KTokensUSD, ccPerUSD, weight float64) error {
	return s.q.UpsertProviderRate(ctx, sqlcgen.UpsertProviderRateParams{
		Provider:            provider,
		PricePer1kTokensUsd: pricePer1KTokensUSD,
		CcPerUsd:            ccPerUSD,
		Weight:              weight,
	})
}

func (s *Store) EnsureDefaultProviderRates(ctx context.Context, defaults []ProviderRate) error {
	for _, r := range defaults {
		if err := s.q.EnsureDefaultProviderRate(ctx, sqlcgen.EnsureDefaultProviderRateParams{
			Provider:            r.Provider,
			PricePer1kTokensUsd: r.PricePer1KTokensUSD,
			CcPerUsd:            r.CCPerUSD,
			Weight:              r.Weight,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAccounts(ctx context.Context, agentID string, limit, offset int) ([]Account, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListAccounts(ctx, sqlcgen.ListAccountsParams{
		Column1: agentID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, Account{
			AgentID:   r.AgentID,
			BalanceCC: r.BalanceCc,
			UpdatedAt: r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) ListLedgerEntries(ctx context.Context, f LedgerFilter, limit, offset int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListLedgerEntries(ctx, sqlcgen.ListLedgerEntriesParams{
		Column1: f.AgentID,
		Column2: f.HandID,
		Column3: timeParam(f.From),
		Column4: timeParam(f.To),
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]LedgerEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, LedgerEntry{
			ID:        r.ID,
			AgentID:   r.AgentID,
			Type:      r.Type,
			AmountCC:  r.AmountCc,
			RefType:   r.RefType,
			RefID:     r.RefID,
			CreatedAt: r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) ListLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListLeaderboard(ctx, sqlcgen.ListLeaderboardParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]LeaderboardEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, LeaderboardEntry{
			AgentID: r.ID,
			Name:    r.Name,
			NetCC:   anyToInt64(r.NetCc),
		})
	}
	return out, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func textParam(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func int4Param(v int32) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: true}
}

func timeParam(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}

func textVal(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func anyToInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func (s *Store) CreateAgentSession(ctx context.Context, sess AgentSession) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO agent_sessions (id, agent_id, room_id, table_id, seat_id, join_mode, status, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)
	`, sess.ID, sess.AgentID, sess.RoomID, sess.TableID, sess.SeatID, sess.JoinMode, sess.Status, sess.ExpiresAt)
	return err
}

func (s *Store) GetAgentSession(ctx context.Context, sessionID string) (*AgentSession, error) {
	var out AgentSession
	var seatID *int
	var tableID *string
	var closedAt *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT id, agent_id, room_id, table_id, seat_id, join_mode, status, expires_at, created_at, closed_at
		FROM agent_sessions
		WHERE id = $1
	`, sessionID).Scan(
		&out.ID, &out.AgentID, &out.RoomID, &tableID, &seatID, &out.JoinMode, &out.Status, &out.ExpiresAt, &out.CreatedAt, &closedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if tableID != nil {
		out.TableID = *tableID
	}
	out.SeatID = seatID
	out.ClosedAt = closedAt
	return &out, nil
}

func (s *Store) UpdateAgentSessionMatch(ctx context.Context, sessionID, tableID string, seatID int) error {
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions
		SET table_id = $2, seat_id = $3, status = 'active'
		WHERE id = $1
	`, sessionID, tableID, seatID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CloseAgentSession(ctx context.Context, sessionID string) error {
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions
		SET status = 'closed', closed_at = now()
		WHERE id = $1 AND status <> 'closed'
	`, sessionID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) InsertAgentActionRequest(ctx context.Context, req AgentActionRequest) (bool, error) {
	if req.ID == "" {
		req.ID = NewID()
	}
	cmd, err := s.Pool.Exec(ctx, `
		INSERT INTO agent_action_requests (id, session_id, request_id, turn_id, action_type, amount_cc, thought_log, accepted, reason)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''))
		ON CONFLICT (session_id, request_id) DO NOTHING
	`, req.ID, req.SessionID, req.RequestID, req.TurnID, req.Action, req.AmountCC, req.ThoughtLog, req.Accepted, req.Reason)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (s *Store) GetAgentActionRequest(ctx context.Context, sessionID, requestID string) (*AgentActionRequest, error) {
	var out AgentActionRequest
	var amount *int64
	var thought *string
	var reason *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, session_id, request_id, turn_id, action_type, amount_cc, thought_log, accepted, reason, created_at
		FROM agent_action_requests
		WHERE session_id = $1 AND request_id = $2
	`, sessionID, requestID).Scan(
		&out.ID, &out.SessionID, &out.RequestID, &out.TurnID, &out.Action, &amount, &thought, &out.Accepted, &reason, &out.CreatedAt,
	)
	if err != nil {
		return nil, mapNotFound(err)
	}
	out.AmountCC = amount
	if thought != nil {
		out.ThoughtLog = *thought
	}
	if reason != nil {
		out.Reason = *reason
	}
	return &out, nil
}

func (s *Store) UpsertAgentEventOffset(ctx context.Context, sessionID, lastEventID string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO agent_event_offsets (session_id, last_event_id)
		VALUES ($1, $2)
		ON CONFLICT (session_id)
		DO UPDATE SET last_event_id = EXCLUDED.last_event_id, updated_at = now()
	`, sessionID, lastEventID)
	return err
}

func (s *Store) GetAgentEventOffset(ctx context.Context, sessionID string) (*AgentEventOffset, error) {
	var out AgentEventOffset
	err := s.Pool.QueryRow(ctx, `
		SELECT session_id, last_event_id, updated_at
		FROM agent_event_offsets
		WHERE session_id = $1
	`, sessionID).Scan(&out.SessionID, &out.LastEventID, &out.UpdatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (s *Store) DebugSessionCount(ctx context.Context) (int, error) {
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_sessions`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return count, nil
}
