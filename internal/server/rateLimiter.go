package server

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"golang.org/x/time/rate"
)

type limiterTimeWrap struct {
	limiter  *rate.Limiter
	lastSeen int64
}

func (s *Server) cleanupLimiters() {
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			now := time.Now().Unix()
			s.userLimiters.Range(func(key, value any) bool {
				ul, ok := value.(*limiterTimeWrap)
				if !ok {
					return true
				}

				if now-atomic.LoadInt64(&ul.lastSeen) > 3600 {
					s.userLimiters.Delete(key)
				}
				return true
			})
		}
	}()
}

func (s *Server) rateLimited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.GetUserFromContext(r.Context())
		if !ok || user.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now().Unix()
		val, exists := s.userLimiters.Load(user.ID)

		var wrap *limiterTimeWrap
		if !exists {
			wrap = &limiterTimeWrap{
				limiter:  rate.NewLimiter(rate.Every(time.Second), 5),
				lastSeen: now,
			}
			actual, _ := s.userLimiters.LoadOrStore(user.ID, wrap)
			wrap = actual.(*limiterTimeWrap)
		} else {
			wrap = val.(*limiterTimeWrap)

			if now-atomic.LoadInt64(&wrap.lastSeen) > 60 {
				atomic.StoreInt64(&wrap.lastSeen, now)
			}
		}

		if !wrap.limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
