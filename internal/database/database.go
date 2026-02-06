package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

func InitDB() *Database {
	const maxRetries = 5
	const baseDelay = 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		config, err := pgxpool.ParseConfig("")
		if err != nil {
			panic(err)
		}

		config.ConnConfig.User = os.Getenv("DB_USER")
		config.ConnConfig.Password = os.Getenv("DB_PASSWORD")
		config.ConnConfig.Host = "/var/run/postgresql"
		config.ConnConfig.Database = os.Getenv("DB_NAME")
		config.ConnConfig.Port = uint16(5432)
		config.ConnConfig.SSLNegotiation = "disable"

		config.MaxConns = 20
		config.MinConns = 5
		config.MaxConnIdleTime = 5 * time.Minute

		pool, err := pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			slog.Warn("DB connect failed", "attempt", attempt, "of", maxRetries, "error", err)
			if attempt == maxRetries {
				fmt.Fprintf(os.Stderr, "Failed to connect after %d attempts: %v\n", maxRetries, err)
				os.Exit(1)
			}
			time.Sleep(baseDelay * time.Duration(1<<attempt))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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
