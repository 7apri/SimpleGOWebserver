package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/redis/go-redis/v9"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- { // from the first one to the h
		h = middlewares[i](h)
	}
	return h
}

func NewAllowMethodMiddleware(methods ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(methods, r.Method) {
				w.Header().Set("Allow", strings.Join(methods, ", "))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(r.Context(), "Panic recovered", "err", err)

				w.Header().Set("Connection", "close")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

var quotaScript = redis.NewScript(`
    local current = redis.call("INCR", KEYS[1])
    if current == 1 then
        redis.call("EXPIRE", KEYS[1], 86400) -- 24 hours in seconds
    end
    return current
`)

func (s *Server) dailyRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.GetUserFromContext(r.Context())
		if !ok || user.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		key := fmt.Sprintf("quota:user:%d:%s", user.ID, time.Now().Format("2006-01-02"))

		limit := 400
		count, err := quotaScript.Run(r.Context(), s.redis, []string{key}).Int()
		if err != nil {
			slog.Error("Redis error rate limiter", "err", err)
			next.ServeHTTP(w, r)
			return
		}

		if count > limit {
			http.Error(w, "Daily quota exceeded", http.StatusTooManyRequests)
			return
		}

		w.Header().Set("X-Daily-Limit", fmt.Sprintf("%d", limit))
		w.Header().Set("X-Daily-Remaining", fmt.Sprintf("%d", limit-count))

		next.ServeHTTP(w, r)
	})
}
