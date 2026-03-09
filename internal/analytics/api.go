package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/mileusna/useragent"
)

type Event struct {
	UserID        *uuid.UUID `json:"uid,omitempty"`
	Path          string     `json:"path"`
	Method        string     `json:"method"`
	Status        int        `json:"status"`
	DurationMicro int64      `json:"dur"`
	IP            string     `json:"ip"`
	UserAgent     string     `json:"user_agent"`
	CreatedAt     time.Time  `json:"ts"`
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

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Microseconds()
		path := r.URL.Path
		method := r.Method
		ip := util.GetClientIP(r)
		rawUA := r.UserAgent()

		var uid *uuid.UUID
		if user, ok := auth.GetUserFromContext(r.Context()); ok {
			uid = &user.ID
		}

		go func(u *uuid.UUID) {
			ua := useragent.Parse(rawUA)

			event := Event{
				UserID:        u,
				Path:          path,
				Method:        method,
				Status:        rw.status,
				DurationMicro: duration,
				IP:            ip,
				UserAgent:     fmt.Sprintf("%s on %s", ua.Name, ua.OS),
				CreatedAt:     time.Now(),
			}

			data, err := sonic.Marshal(event)
			if err != nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := s.redis.RPush(ctx, "analytics_queue", data).Err(); err != nil {
				slog.Error("Failed to push analytics", "err", err)
			}
		}(uid)
	})
}
