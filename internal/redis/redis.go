package redis

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/redis/go-redis/v9"
)

func Init(ctx context.Context, redisAddr, redisPassword string) *redis.Client {
	var rdb *redis.Client

	for attempt := 1; attempt <= consts.DatabaseMaxConnectRetries; attempt++ {
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
		if attempt == consts.DatabaseMaxConnectRetries {
			slog.Error("redis connection failed", "err", err, "addr", redisAddr, "pass", redisPassword)
			os.Exit(1)
		}
		slog.Warn("redis connection failed", "err", err, "attempt", attempt)
		time.Sleep(consts.DatabaseConnectBaseDelay * time.Duration(1<<attempt))
	}
	return rdb
}
