package web

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

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
	new := WebError{
		Status: status,
		Key:    key,
		Err:    err,
		Data:   data,
	}
	if new.Key == "" {
		new.Key = new.Err.Error()
	}
	return &new
}

func (e *WebError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%d (%s): %v | data: %v", e.Status, e.Key, e.Err, e.Data)
	}
	return fmt.Sprintf("%d (%s) | data: %v", e.Status, e.Key, e.Data)
}

type Handler func(w http.ResponseWriter, r *http.Request) *WebError
type OnErrHtmlFunc func(w http.ResponseWriter, r *http.Request, buffer *bytes.Buffer, appErr *WebError) *WebError

type BufferPool struct {
	pool    sync.Pool
	maxSize int
}

func NewBufferPool(maxSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		},
		maxSize: maxSize,
	}
}

func (p *BufferPool) Get() *bytes.Buffer {
	b := p.pool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	if b.Cap() <= p.maxSize {
		p.pool.Put(b)
	}
}

var bufferPool = NewBufferPool(64 * 1024)

type ContentType uint8

const (
	TypeHTML ContentType = iota + 1
	TypePlain
	TypeJSON
)

type ctxKeyAccept uint8

const AcceptKey ctxKeyAccept = 0

func GetAcceptHeaderCtx(ctx context.Context) (ContentType, bool) {
	if ctx == nil {
		return 0, false
	}
	val := ctx.Value(AcceptKey)
	accept, ok := val.(ContentType)
	return accept, ok
}
func GetAcceptHeaderReq(r *http.Request) ContentType {
	ctx := r.Context()
	accept, ok := GetAcceptHeaderCtx(ctx)
	if !ok {
		accept = ParseAcceptHeader(r)
	}
	return accept
}
func ParseAcceptHeader(r *http.Request) ContentType {
	accept := r.Header.Get("Accept")

	resolved := TypeHTML

	if strings.Contains(accept, "application/json") {
		resolved = TypeJSON
	} else if strings.Contains(accept, "text/plain") && !strings.Contains(accept, "text/html") {
		resolved = TypePlain
	}

	return resolved
}
func AcceptMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), AcceptKey, ParseAcceptHeader(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func DefaultErrHtml(w http.ResponseWriter, r *http.Request, b *bytes.Buffer, appErr *WebError) *WebError {
	b.WriteString("<h1>Error</h1><p>" + appErr.Key + "</p>")
	return nil
}

func writePlain(w http.ResponseWriter, resp *bytes.Buffer, msg string) {
	resp.Reset()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.WriteString(msg)
}
func MakeHandler(h Handler, i18nMgr *i18n.I18nManager, onErrHtml OnErrHtmlFunc) http.HandlerFunc {
	if onErrHtml == nil {
		onErrHtml = DefaultErrHtml
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if appErr := h(w, r); appErr != nil {

			isServerError := appErr.Status >= 500
			isImportantClientError := appErr.Status != 404 && appErr.Status >= 400

			if isServerError || isImportantClientError || appErr.Err != nil {
				slog.ErrorContext(r.Context(), "request_failed",
					"status", appErr.Status,
					"key", appErr.Key,
					"err", appErr.Err,
					"data", appErr.Data,
					"path", r.URL.Path,
				)
			}

			lang := i18n.GetLangFromReq(r)
			msg, err := i18nMgr.TranslateError(lang, appErr.Key)
			if err != nil {
				msg = appErr.Key
			}

			resp := bufferPool.Get()
			defer bufferPool.Put(resp)

			accepts := GetAcceptHeaderReq(r)

			switch accepts {
			case TypeJSON:
				w.Header().Set("Content-Type", "application/json; charset=utf-8")

				jsonBody := map[string]any{
					"error": msg,
					"code":  appErr.Key,
				}
				if appErr.Data != nil {
					jsonBody["data"] = appErr.Data
				}

				if err := sonic.ConfigDefault.NewEncoder(resp).Encode(jsonBody); err != nil {
					resp.WriteString(`{"error":"internal error","code":"err_internal"}`)
				}
			case TypeHTML:
				if err := onErrHtml(w, r, resp, appErr); err != nil {
					writePlain(w, resp, msg)
				}
			default:
				writePlain(w, resp, msg)
			}

			w.WriteHeader(appErr.Status)
			w.Write(resp.Bytes())
		}
	}
}

func (h Handler) With(i18nMgr *i18n.I18nManager, onErrHtml OnErrHtmlFunc, mw ...Middleware) http.Handler {
	return Chain(MakeHandler(h, i18nMgr, onErrHtml), mw...)
}
