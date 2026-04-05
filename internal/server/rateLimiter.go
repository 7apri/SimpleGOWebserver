package server

import (
	"bytes"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/web"
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

func (s *Server) rateLimited(key string, limit rate.Limit, b int) web.Middleware {
	return func(next http.Handler) http.Handler {
		return web.MakeHandler(func(w http.ResponseWriter, r *http.Request) *web.WebError {
			key := key

			user, ok := auth.GetUserFromContext(r.Context())
			if !ok {
				key += web.GetClientIP(r)
			} else if user.Role == "dev" {
				next.ServeHTTP(w, r)
				return nil
			} else {
				key += user.ID.String()
			}

			now := time.Now().Unix()

			val, exists := s.userLimiters.Load(key)

			var wrap *limiterTimeWrap
			var isAllowed = true
			if !exists {
				wrap = &limiterTimeWrap{
					limiter:  rate.NewLimiter(limit, b),
					lastSeen: now,
				}
				actual, _ := s.userLimiters.LoadOrStore(key, wrap)
				wrap = actual.(*limiterTimeWrap)
			} else {
				wrap = val.(*limiterTimeWrap)
				isAllowed = wrap.limiter.Allow()
			}

			atomic.StoreInt64(&wrap.lastSeen, now)

			if !isAllowed {
				return web.NewError(http.StatusTooManyRequests, "err_too_many_requests", nil, nil)
			}

			next.ServeHTTP(w, r)
			return nil
		}, s.i18Mgr, func(w http.ResponseWriter, r *http.Request, buffer *bytes.Buffer, appErr *web.WebError) *web.WebError {
			return appErr
		})
	}
}
