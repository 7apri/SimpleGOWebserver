package util

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

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
