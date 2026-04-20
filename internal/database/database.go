package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

func Init(ctx context.Context, user, password, databaseName string) *Database {
	config, err := pgxpool.ParseConfig("")
	if err != nil {
		panic(err)
	}

	config.ConnConfig.User = user
	config.ConnConfig.Password = password
	config.ConnConfig.Host = "/var/run/postgresql"
	config.ConnConfig.Database = databaseName
	config.ConnConfig.Port = uint16(5432)
	config.ConnConfig.SSLNegotiation = "disable"

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnIdleTime = 5 * time.Minute

	for attempt := 1; attempt <= consts.DatabaseMaxConnectRetries; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			slog.Warn("DB connect failed", "attempt", attempt, "of", consts.DatabaseMaxConnectRetries, "error", err)
			if attempt == consts.DatabaseMaxConnectRetries {
				fmt.Fprintf(os.Stderr, "Failed to connect after %d attempts: %v\n", consts.DatabaseMaxConnectRetries, err)
				os.Exit(1)
			}
			time.Sleep(consts.DatabaseConnectBaseDelay * time.Duration(1<<attempt))
			continue
		}

		if err := pool.Ping(ctx); err != nil {
			slog.Warn("DB ping failed", "attempt", attempt, "error", err)
			pool.Close()
			if attempt == consts.DatabaseMaxConnectRetries {
				fmt.Fprintf(os.Stderr, "DB ping failed after connect: %v\n", err)
				os.Exit(1)
			}
			time.Sleep(consts.DatabaseConnectBaseDelay * time.Duration(1<<attempt))
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
