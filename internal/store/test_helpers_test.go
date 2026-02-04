package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silicon-casino/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func openStore(t *testing.T) (*Store, context.Context, func()) {
	t.Helper()
	cfg, err := config.LoadTest()
	if err != nil {
		t.Skipf("skip test db: %v", err)
	}
	dsn := cfg.TestPostgresDSN
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	base, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open base db: %v", err)
	}
	if _, err := base.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		base.Close()
		t.Fatalf("create schema: %v", err)
	}
	base.Close()

	st, err := New(withSearchPath(dsn, schema))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := applySchema(st); err != nil {
		st.Close()
		t.Fatalf("apply schema: %v", err)
	}
	cleanup := func() {
		st.Close()
		base, err := pgxpool.New(context.Background(), dsn)
		if err == nil {
			_, _ = base.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
			base.Close()
		}
	}
	return st, context.Background(), cleanup
}

func applySchema(st *Store) error {
	path, err := findInitMigrationPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = st.Pool.Exec(context.Background(), string(b))
	return err
}

func findInitMigrationPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "migrations", "000001_init.up.sql")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("000001_init.up.sql not found from %s", dir)
}

func withSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema
}

func mustCreateAgent(t *testing.T, st *Store, ctx context.Context, name, apiKey string, initial int64) string {
	t.Helper()
	id, err := st.CreateAgent(ctx, name, apiKey)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := st.EnsureAccount(ctx, id, initial); err != nil {
		t.Fatalf("ensure account: %v", err)
	}
	return id
}
