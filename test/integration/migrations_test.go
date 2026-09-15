//go:build integration

package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsSourceURL returns a golang-migrate file source URL for the
// migrations directory, relative to this package's location.
func migrationsSourceURL() string {
	abs, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		return ""
	}
	return "file://" + strings.ReplaceAll(abs, "\\", "/")
}

// testMigrator starts a real Postgres container via testcontainers, applies
// migrations, and returns the migrator (no external DB or credentials needed).
func testMigrator(t *testing.T) (*migrate.Migrate, *sql.DB) {
	t.Helper()

	pg, err := postgres.Run(context.Background(),
		"postgres:16-alpine",
		postgres.WithDatabase("finance_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = pg.Terminate(context.Background())
	})

	dsn, err := pg.ConnectionString(context.Background(), "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	srcURL := migrationsSourceURL()
	if srcURL == "" {
		t.Fatal("could not resolve migrations directory")
	}
	if _, err := os.Stat(strings.TrimPrefix(srcURL, "file://")); err != nil {
		t.Fatalf("migrations dir not found: %v", err)
	}

	m, err := migrate.New(srcURL, dsn)
	if err != nil {
		t.Fatalf("new migrate: %v", err)
	}
	t.Cleanup(func() { m.Close() })

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return m, db
}

func TestMigrationsRunUpAndDown(t *testing.T) {
	m, db := testMigrator(t)

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	for _, table := range []string{
		"companies", "price_history", "income_statements",
		"balance_sheets", "cash_flow_statements", "ingestion_runs",
	} {
		var exists bool
		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist after migrate up", table)
		}
	}

	if err := m.Down(); err != nil {
		t.Fatalf("migrate down: %v", err)
	}

	var exists bool
	if err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'companies')",
	).Scan(&exists); err != nil {
		t.Fatalf("check after down: %v", err)
	}
	if exists {
		t.Error("companies table still exists after migrate down")
	}
}

func TestUniqueConstraintBlocksDuplicateInsert(t *testing.T) {
	m, db := testMigrator(t)

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	var companyID int64
	err := db.QueryRow(
		`INSERT INTO companies (symbol, name) VALUES ('AAPL', 'Apple Inc.') RETURNING id`,
	).Scan(&companyID)
	if err != nil {
		t.Fatalf("insert company: %v", err)
	}

	insertPrice := func() error {
		_, err := db.Exec(
			`INSERT INTO price_history (company_id, date, open, high, low, close, volume)
			 VALUES ($1, '2024-01-02', 190.0, 192.5, 189.0, 191.2, 80000000)`,
			companyID,
		)
		return err
	}

	if err := insertPrice(); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	if err := insertPrice(); err == nil {
		t.Fatal("duplicate insert should have failed")
	} else if !isUniqueViolation(err) {
		t.Fatalf("expected unique violation, got: %v", err)
	}
}

func isUniqueViolation(err error) bool {
	if pe, ok := err.(*pq.Error); ok {
		return pe.Code == "23505"
	}
	return false
}
