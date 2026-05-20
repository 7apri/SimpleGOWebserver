package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

const maxRetries = 5
const baseDelay = 1 * time.Second

func Init(ctx context.Context) *Database {

	config, err := pgxpool.ParseConfig("")
	if err != nil {
		panic(err)
	}

	config.ConnConfig.User = util.TryGetEnvFatal("DB_USER")
	config.ConnConfig.Password = util.TryGetEnvFatal("DB_PASSWORD")
	config.ConnConfig.Host = util.TryGetEnvFatal("DB_SOCKET_PATH")
	config.ConnConfig.Database = util.TryGetEnvFatal("DB_NAME")
	config.ConnConfig.Port = uint16(5432)
	config.ConnConfig.SSLNegotiation = "disable"

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnIdleTime = 5 * time.Minute

	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			slog.Warn("DB connect failed", "attempt", attempt, "of", maxRetries, "error", err)
			if attempt == maxRetries {
				fmt.Fprintf(os.Stderr, "Failed to connect after %d attempts: %v\n", maxRetries, err)
				os.Exit(1)
			}
			time.Sleep(baseDelay * time.Duration(1<<attempt))
			continue
		}

		if err := pool.Ping(ctx); err != nil {
			slog.Warn("DB ping failed", "attempt", attempt, "error", err)
			pool.Close()
			if attempt == maxRetries {
				fmt.Fprintf(os.Stderr, "DB ping failed after connect: %v\n", err)
				os.Exit(1)
			}
			time.Sleep(baseDelay * time.Duration(1<<attempt))
			continue
		}

		slog.Info("Successfully connected to Postgres!", "attempt", attempt)
		return &Database{pool}
	}

	return nil
}

func (db *Database) GetLatency() (string, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Pool.Ping(ctx); err != nil {
		return "", err
	}

	return time.Since(start).String(), nil
}
