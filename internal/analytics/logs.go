package analytics

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
)

type Handler struct {
	service *Service
	console slog.Handler
}

type Log struct {
	Level     string          `json:"level"`
	Message   string          `json:"message"`
	Context   json.RawMessage `json:"context,omitempty"`
	CreatedAt time.Time       `json:"ts"`
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{console: h.console.WithAttrs(attrs), service: h.service}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{console: h.console.WithGroup(name), service: h.service}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	err := h.console.Handle(ctx, r)

	if r.Level >= slog.LevelWarn {
		go func(record slog.Record) {
			data := h.prepareLogData(record)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			h.service.redis.RPush(ctx, "logs_queue", data)
		}(r)
	}
	return err
}

func (h *Handler) prepareLogData(r slog.Record) (data []byte) {
	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	var (
		err         error
		contextJSON []byte
	)

	contextJSON, err = sonic.Marshal(attrs)
	if err != nil {
		log.Printf("INTERNAL_ERROR: Marshal of a record's context failed: %s", err)
	}
	err = nil

	data, err = sonic.Marshal(
		Log{
			Level:     r.Level.String(),
			Message:   r.Message,
			Context:   contextJSON,
			CreatedAt: time.Now(),
		},
	)
	if err != nil {
		log.Printf("INTERNAL_ERROR: Marshal of a record failed: %s", err)
	}

	return data
}
