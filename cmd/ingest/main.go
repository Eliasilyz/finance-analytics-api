package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/provider"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"

	_ "github.com/lib/pq"
)

func main() {
	symbols := flag.String("symbols", "AAPL,MSFT", "comma-separated symbols to ingest")
	delay := flag.Duration("delay", 15*time.Second, "pause between symbols (Alpha Vantage free tier ~5 req/min, ~25/day; one symbol = 5 endpoints)")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("ALPHA_VANTAGE_API_KEY")
	if dbURL == "" || apiKey == "" {
		log.Fatal("DATABASE_URL and ALPHA_VANTAGE_API_KEY are required")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Println("connected to postgres")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := service.NewIngestionService(
		provider.NewAlphaVantage(apiKey),
		repository.NewRepository(db),
		logger,
	)

	failed := false
	for _, sym := range strings.Split(*symbols, ",") {
		sym = strings.TrimSpace(sym)
		if sym == "" || ctx.Err() != nil {
			continue
		}
		run, err := svc.Ingest(ctx, sym)
		if err != nil {
			logger.Error("ingest failed", "symbol", sym, "error", err)
			failed = true
		} else {
			logger.Info("ingest ok",
				"symbol", sym,
				"status", run.Status,
				"prices", run.PricesInserted,
				"income", run.IncomeInserted,
				"balance", run.BalanceInserted,
				"cash_flow", run.CashFlowInserted,
			)
		}
		time.Sleep(*delay)
	}
	if failed {
		os.Exit(1)
	}
}
