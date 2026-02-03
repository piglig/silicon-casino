package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silicon-casino/internal/store"
)

func OpenTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	base, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open base db: %v", err)
	}
	if _, err := base.Exec(`CREATE SCHEMA ` + schema); err != nil {
		_ = base.Close()
		t.Fatalf("create schema: %v", err)
	}
	_ = base.Close()

	dsnWithSchema := withSearchPath(dsn, schema)
	st, err := store.New(dsnWithSchema)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := applySchema(st); err != nil {
		_ = st.DB.Close()
		t.Fatalf("apply schema: %v", err)
	}

	cleanup := func() {
		_ = st.DB.Close()
		base, err := sql.Open("pgx", dsn)
		if err == nil {
			_, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
			_ = base.Close()
		}
	}
	return st, cleanup
}

func applySchema(st *store.Store) error {
	path, err := findSchemaPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = st.DB.Exec(string(b))
	return err
}

func findSchemaPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "internal", "store", "schema.sql")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("schema.sql not found from %s", dir)
}

func withSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema
}
