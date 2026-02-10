package analytics

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type Event struct {
	UserID     *int64    `json:"uid,omitempty"`
	Path       string    `json:"path"`
	Method     string    `json:"method"`
	Status     int       `json:"status"`
	DurationMS int64     `json:"dur"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"ts"`
}

type Service struct {
	db    *database.Database
	redis *redis.Client
}

func NewService(ctx context.Context, db *database.Database, rdb *redis.Client) *Service {
	sv := Service{db: db, redis: rdb}
	go sv.StartWorker(ctx)
	return &sv
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = 200
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Milliseconds()

		var uid *int64
		if user, ok := auth.GetUserFromContext(r.Context()); ok {
			uid = &user.ID
		}

		event := Event{
			UserID:     uid,
			Path:       r.URL.Path,
			Method:     r.Method,
			Status:     rw.status,
			DurationMS: duration,
			IP:         r.RemoteAddr,
			CreatedAt:  time.Now(),
		}

		data, _ := json.Marshal(event)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			s.redis.RPush(ctx, "analytics_queue", data)
		}()
	})
}

func (s *Service) StartWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flush(context.Background())
			return
		case <-ticker.C:
			s.flush(ctx)
		}
	}
}

func (s *Service) flush(ctx context.Context) {
	results, err := s.redis.LPopCount(ctx, "analytics_queue", 150).Result()
	if err != nil {
		if err != redis.Nil {
			slog.Error("Redis analytics pop failed", "err", err)
		}
		return
	}

	if len(results) == 0 {
		return
	}

	rows := make([][]any, 0, len(results))
	for _, val := range results {
		var e Event
		if err := json.Unmarshal([]byte(val), &e); err == nil {
			rows = append(rows, []any{e.UserID, e.Path, e.Method, e.Status, e.DurationMS, e.IP, e.CreatedAt})
		}
	}

	_, err = s.db.Pool.CopyFrom(
		ctx,
		pgx.Identifier{"analytics"},
		[]string{"user_id", "path", "method", "status", "duration_ms", "ip", "created_at"},
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		slog.Error("Failed to flush analytics to DB", "err", err)
	} else {
		slog.Info("Flushed analytics", "count", len(rows))
	}
}
