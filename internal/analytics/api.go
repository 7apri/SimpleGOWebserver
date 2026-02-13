package analytics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type Event struct {
	UserID     *uuid.UUID `json:"uid,omitempty"`
	Path       string     `json:"path"`
	Method     string     `json:"method"`
	Status     int        `json:"status"`
	DurationMS int64      `json:"dur"`
	IP         string     `json:"ip"`
	CreatedAt  time.Time  `json:"ts"`
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

		var uid *uuid.UUID
		if user, ok := auth.GetUserFromContext(r.Context()); ok {
			uid = &user.ID
		}

		event := Event{
			UserID:     uid,
			Path:       r.URL.Path,
			Method:     r.Method,
			Status:     rw.status,
			DurationMS: duration,
			IP:         util.GetClientIP(r),
			CreatedAt:  time.Now(),
		}

		go func() {
			data, _ := sonic.Marshal(event)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := s.redis.RPush(ctx, "analytics_queue", data).Err(); err != nil {
				slog.Error("Failed to push analytics", "err", err)
			}
		}()
	})
}
