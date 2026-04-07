package analytics

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func (s *Service) runFlushCycle() {
	s.flush("analytics_queue", "analytics",
		[]string{"user_id", "path", "method", "status", "duration_micro", "ip", "user_agent", "created_at"},
		func(val string) ([]any, error) {
			var e Event
			if err := sonic.UnmarshalString(val, &e); err != nil {
				return nil, err
			}
			return []any{e.UserID, e.Path, e.Method, e.Status, e.DurationMicro, e.IP, e.UserAgent, e.CreatedAt}, nil
		},
	)

	s.flush("logs_queue", "logs",
		[]string{"level", "message", "context", "created_at"},
		func(val string) ([]any, error) {
			var l Log
			if err := sonic.UnmarshalString(val, &l); err != nil {
				return nil, err
			}
			return []any{l.Level, l.Message, l.Context, l.CreatedAt}, nil
		},
	)
}

func (s *Service) flush(redisKey, tableName string, columns []string, mapper func(string) ([]any, error)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := s.redis.LPopCount(ctx, redisKey, 150).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Error("Redis pop failed", "key", redisKey, "err", err)
		}
		return
	}
	if len(results) == 0 {
		return
	}

	rows := make([][]any, 0, len(results))
	for _, val := range results {
		row, err := mapper(val)
		if err != nil {
			slog.Warn("Failed to unmarshal item, skipping", "key", redisKey, "err", err)
			continue
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return
	}

	_, err = s.db.Pool.CopyFrom(
		ctx,
		pgx.Identifier{tableName},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		slog.Error("CRITICAL: Flush to DB failed. Data lost. (and we do not care)", "table", tableName, "err", err, "count", len(rows))
	} else {
		slog.Info("Flushed batch", "table", tableName, "count", len(rows))
	}
}
