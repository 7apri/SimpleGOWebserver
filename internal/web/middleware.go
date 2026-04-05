package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- { // from the first one to the h
		h = middlewares[i](h)
	}
	return h
}
func RecoveryM(next http.Handler) http.Handler {
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
func QuantizeDelay(target time.Duration, jitterRange int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			next.ServeHTTP(w, r)

			elapsed := time.Since(start)
			if elapsed < target {
				jitter := time.Duration(util.RandomInt(jitterRange)) * time.Millisecond
				time.Sleep(target - elapsed + jitter)
			}
		})
	}
}
