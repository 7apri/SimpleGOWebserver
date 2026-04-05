package redis

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/redis/go-redis/v9"
)

const maxRetries = 5
const baseDelay = 1 * time.Second

func Init(ctx context.Context) *redis.Client {
	redisAddr := util.TryGetEnvFatal("REDIS_ADDRESS")
	redisPassword := util.TryGetEnvFatal("REDIS_PASSWORD")

	var rdb *redis.Client

	for attempt := 1; attempt <= maxRetries; attempt++ {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       0,
		})
		err := rdb.Ping(ctx).Err()
		if err == nil {
			slog.Info("Successfully connected to redis!", "attempt", attempt)
			break
		}
		if attempt == maxRetries {
			slog.Error("redis connection failed", "err", err, "addr", redisAddr, "pass", redisPassword)
			os.Exit(1)
		}
		slog.Warn("redis connection failed", "err", err, "attempt", attempt)
		time.Sleep(baseDelay * time.Duration(1<<attempt))
	}
	return rdb
}
