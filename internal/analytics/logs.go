package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

type Handler struct {
	out     io.Writer
	level   slog.Level
	service *Service
	attrs   []slog.Attr
	groups  []string
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

var levelColors = map[slog.Level]string{
	slog.LevelDebug: "\033[34m", // Blue
	slog.LevelInfo:  "\033[32m", // Green
	slog.LevelWarn:  "\033[33m", // Yellow
	slog.LevelError: "\033[31m", // Red
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	timeStr := r.Time.Format("15:04:05")

	color := levelColors[r.Level]

	fmt.Fprintf(buf, "\033[2m%s\033[0m ", timeStr)
	fmt.Fprintf(buf, "%s[%s] \033[0m", color, r.Level)
	fmt.Fprintf(buf, "\033[1m%s\033[0m", r.Message)

	if len(h.attrs) > 0 || r.NumAttrs() > 0 {
		buf.WriteString(" \033[34m→\033[0m ")

		for _, a := range h.attrs {
			fmt.Fprintf(buf, " %s=%v", a.Key, a.Value)
		}

		r.Attrs(func(a slog.Attr) bool {
			fmt.Fprintf(buf, " %s=%v", a.Key, a.Value)
			return true
		})
	}
	buf.WriteByte('\n')

	_, err := h.out.Write(buf.Bytes())

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

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		out:     h.out,
		level:   h.level,
		service: h.service,
		attrs:   append(h.attrs, attrs...),
		groups:  h.groups,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		out:     h.out,
		level:   h.level,
		service: h.service,
		attrs:   h.attrs,
		groups:  append(h.groups, name),
	}
}

type Log struct {
	Level     string          `json:"level"`
	Message   string          `json:"message"`
	Context   json.RawMessage `json:"context,omitempty"`
	CreatedAt time.Time       `json:"ts"`
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
