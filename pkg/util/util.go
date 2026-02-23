package util

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

type ReadOnlyMap[K comparable, V any] struct {
	data map[K]V
}

func NewReadOnlyMap[K comparable, V any](m map[K]V) ReadOnlyMap[K, V] {
	return ReadOnlyMap[K, V]{m}
}

func (w ReadOnlyMap[K, V]) Lookup(id K) (V, bool) {
	val, ok := w.data[id]
	return val, ok
}
func (w ReadOnlyMap[K, V]) Len() int {
	return len(w.data)
}
func (w ReadOnlyMap[K, V]) Get(id K) V {
	return w.data[id]
}
func (w ReadOnlyMap[K, V]) All() map[K]V {
	return w.data
}

func PingGoogle() (string, error) {
	start := time.Now()

	resp, err := http.Head("https://www.google.com")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return time.Since(start).String(), nil
}

func FilterNil[T any](data []*T) []*T {
	n := 0
	for _, x := range data {
		if x != nil {
			data[n] = x
			n++
		}
	}
	return data[:n]
}
func TryGetEnvFatal(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("please check the .env", "missing key", k)
		os.Exit(1)
	}
	return v
}
