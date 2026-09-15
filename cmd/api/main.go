package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/eliasilyz/finance-analytics-api/internal/handler"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

func main() {
	dbURL := mustEnv("DATABASE_URL")
	redisURL := mustEnv("REDIS_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Println("connected to postgres")

	rdbOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("redis url parse failed: %v", err)
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	log.Println("connected to redis")

	repo := repository.NewRepository(db)
	qs := service.NewQueryService(repo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	h := handler.New(qs, logger)
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
