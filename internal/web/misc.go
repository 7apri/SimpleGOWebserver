package web

import (
	"net"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

func SendJSON(w http.ResponseWriter, status int, data any) *WebError {
	w.Header().Set("Content-Type", "application/json")
	err := sonic.ConfigDefault.NewEncoder(w).Encode(data)
	if err != nil {
		return NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}
func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		part := strings.Split(xff, ",")[0]
		return strings.TrimSpace(part)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
