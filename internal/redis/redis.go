package redis

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/redis/go-redis/v9"
)

const (
	maxRetries = 5
	baseDelay  = 1 * time.Second
)

func Init(ctx context.Context) *redis.Client {
	redisAddr := util.TryGetEnvFatal("REDIS_ADDRESS")
	redisPassword := util.TryGetEnvFatal("REDIS_PASSWORD")

	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           0,
		MaxRetries:   -1,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	for attempt := 1; attempt <= maxRetries; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := rdb.Ping(pingCtx).Err()
		cancel()

		if err == nil {
			slog.Info("Successfully connected to redis!", "attempt", attempt)
			return rdb
		}

		if attempt == maxRetries {
			slog.Error("redis connection failed permanently", "err", err, "addr", redisAddr)
			rdb.Close()
			os.Exit(1)
		}

		slog.Warn("redis connection failed, retrying...", "err", err, "attempt", attempt)

		time.Sleep(baseDelay * time.Duration(1<<(attempt-1)))
	}

	return rdb
}
