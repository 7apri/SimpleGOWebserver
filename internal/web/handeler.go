package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/bytedance/sonic"
)

type WebError struct {
	Status int
	Key    string
	Err    error
	Data   any
}

func NewError(status int, key string, err error, data any) *WebError {
	return &WebError{
		Status: status,
		Key:    key,
		Err:    err,
		Data:   data,
	}
}

func (e *WebError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%d (%s): %v | data: %v", e.Status, e.Key, e.Err, e.Data)
	}
	return fmt.Sprintf("%d (%s) | data: %v", e.Status, e.Key, e.Data)
}

type Handler func(w http.ResponseWriter, r *http.Request) *WebError

func MakeHandler(h Handler, i18nMgr *i18n.I18nManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appErr := h(w, r); appErr != nil {
			slog.ErrorContext(r.Context(), "request_failed",
				"status", appErr.Status,
				"key", appErr.Key,
				"err", appErr.Err,
				"data", appErr.Data,
			)

			lang := i18n.GetLangFromReq(r)
			msg, _ := i18nMgr.Translate(lang, appErr.Key)

			var resp []byte

			if strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")

				jsonBody := map[string]any{
					"error": msg,
					"code":  appErr.Key,
				}
				if appErr.Data != nil {
					jsonBody["data"] = appErr.Data
				}

				if b, err := sonic.ConfigDefault.Marshal(jsonBody); err == nil {
					resp = b
				} else {
					resp = []byte(`{"error":"internal error","code":"err_internal"}`)
				}
			} else {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				resp = []byte(msg)
			}

			w.WriteHeader(appErr.Status)
			w.Write(resp)
		}
	}
}

func (h Handler) With(i18nMgr *i18n.I18nManager, mw ...Middleware) http.Handler {
	return Chain(MakeHandler(h, i18nMgr), mw...)
}
