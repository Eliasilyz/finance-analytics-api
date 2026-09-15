package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/eliasilyz/finance-analytics-api/internal/auth"
	"github.com/eliasilyz/finance-analytics-api/internal/cache"
	"github.com/eliasilyz/finance-analytics-api/internal/handler"
	"github.com/eliasilyz/finance-analytics-api/internal/rate"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/eliasilyz/finance-analytics-api/migrations"
)

// migrateUp applies pending embedded migrations. It is idempotent: a restart
// against an already-migrated database is a no-op (ErrNoChange), which is
// exactly what `docker compose up` on a warmed volume needs.
func migrateUp(dbURL string) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("migrate source setup failed: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		log.Fatalf("migrate setup failed: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate up failed: %v", err)
	}
	log.Println("migrations applied (or already up to date)")
}

func main() {
	dbURL := mustEnv("DATABASE_URL")
	redisURL := mustEnv("REDIS_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Println("connected to postgres")

	migrateUp(dbURL)

	rdbOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("redis url parse failed: %v", err)
	}
	// Short timeouts so the fail-open paths (cache miss, rate limiter down)
	// fall back to the DB quickly instead of hanging on a dead Redis.
	rdbOpts.DialTimeout = 300 * time.Millisecond
	rdbOpts.ReadTimeout = 300 * time.Millisecond
	rdbOpts.WriteTimeout = 300 * time.Millisecond
	rdb := redis.NewClient(rdbOpts)
	defer func() { _ = rdb.Close() }()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	log.Println("connected to redis")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	repo := repository.NewRepository(db)
	qs := service.NewQueryService(repo, cache.NewRedis(rdb, 15*time.Minute))
	authSvc := auth.NewService(repo, logger)
	limiter := rate.NewRedis(rdb,
		int64(envInt("RATE_LIMIT_LIMIT", 100)),
		time.Duration(envInt("RATE_LIMIT_WINDOW_SECONDS", 60))*time.Second)
	// Loose per-IP guard for failed-auth floods only. Threshold is far above
	// the per-key limit on purpose: it is a safety net, not rate limiting.
	authGuard := rate.NewRedis(rdb,
		int64(envInt("AUTH_GUARD_IP_LIMIT", 500)),
		time.Duration(envInt("AUTH_GUARD_IP_WINDOW_SECONDS", 60))*time.Second)

	h := handler.New(qs, logger, handler.Options{
		Auth:      authSvc,
		Limiter:   limiter,
		AuthGuard: authGuard,
	})
	r := gin.Default()
	h.Routes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
