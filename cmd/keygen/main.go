// Command keygen manages API keys for the finance-analytics-api service.
//
// Subcommands:
//
//	keygen create --label <label> [--expires-in <duration>]
//	keygen list
//	keygen revoke --id <id>
//
// The raw key is printed exactly once at creation; only its SHA-256 hash is
// stored, so it cannot be recovered later.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/eliasilyz/finance-analytics-api/internal/auth"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer func() { _ = db.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	svc := auth.NewService(repository.NewRepository(db), logger)

	switch os.Args[1] {
	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		label := fs.String("label", "", "human-readable owner label")
		expiresIn := fs.String("expires-in", "", "optional TTL, e.g. 365d")
		fs.Usage = func() { fmt.Fprintln(os.Stderr, "usage: keygen create --label NAME [--expires-in DURATION]") }
		_ = fs.Parse(os.Args[2:])
		if *label == "" {
			log.Fatal("--label is required")
		}
		var exp *time.Time
		if *expiresIn != "" {
			d, err := time.ParseDuration(*expiresIn)
			if err != nil {
				log.Fatalf("bad --expires-in: %v", err)
			}
			t := time.Now().Add(d)
			exp = &t
		}
		raw, key, err := svc.Create(ctx, *label, exp)
		if err != nil {
			log.Fatalf("create failed: %v", err)
		}
		fmt.Printf("key id:     %d\nlabel:      %s\nkey:        %s\n", key.ID, key.Label, raw)
		fmt.Println("store this key now; it will never be shown again")

	case "list":
		keys, err := svc.List(ctx)
		if err != nil {
			log.Fatalf("list failed: %v", err)
		}
		if len(keys) == 0 {
			fmt.Println("no keys")
			return
		}
		fmt.Println("id\tprefix\tlabel\tcreated\t\texpires\t\t\t\t\trevoked\t\t\t\t")
		for _, k := range keys {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
				k.ID, k.KeyPrefix, k.Label,
				k.CreatedAt.Format(time.RFC3339),
				formatTime(k.ExpiresAt),
				formatTime(k.RevokedAt))
		}

	case "revoke":
		fs := flag.NewFlagSet("revoke", flag.ExitOnError)
		id := fs.Int64("id", 0, "key id to revoke")
		fs.Usage = func() { fmt.Fprintln(os.Stderr, "usage: keygen revoke --id ID") }
		_ = fs.Parse(os.Args[2:])
		if *id == 0 {
			log.Fatal("--id is required")
		}
		if err := svc.Revoke(ctx, *id); err != nil {
			log.Fatalf("revoke failed: %v", err)
		}
		fmt.Printf("key %d revoked\n", *id)

	default:
		usage()
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "(never)"
	}
	return t.Format(time.RFC3339)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: keygen <create|list|revoke> [flags]")
	os.Exit(2)
}
