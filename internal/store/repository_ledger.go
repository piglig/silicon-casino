package store

import (
	"context"
	"time"

	"silicon-casino/internal/store/sqlcgen"
)

type LedgerFilter struct {
	AgentID string
	HandID  string
	From    *time.Time
	To      *time.Time
}

func (s *Store) ListLedgerEntries(ctx context.Context, f LedgerFilter, limit, offset int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListLedgerEntries(ctx, sqlcgen.ListLedgerEntriesParams{
		AgentID:    f.AgentID,
		HandID:     f.HandID,
		FromTs:     timeParam(f.From),
		ToTs:       timeParam(f.To),
		LimitRows:  int32(limit),
		OffsetRows: int32(offset),
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
