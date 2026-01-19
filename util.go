package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func sendJson(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Failed to marshal json payload: %v", payload)
		w.WriteHeader(500)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

type APIError struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func sendErrorJson(w http.ResponseWriter, message string, code int) {
	sendJson(w, code, APIError{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

func parseGenericQuery[T any](query url.Values, mapper func([]string) T, keys ...string) []T {
	if len(keys) == 0 {
		return nil
	}

	allSlices := make([][]string, len(keys))
	minLen := -1

	for i, key := range keys {
		val := query.Get(key)
		if val == "" {
			return nil
		}

		parts := strings.Split(val, ",")
		allSlices[i] = parts

		if minLen == -1 || len(parts) < minLen {
			minLen = len(parts)
		}
	}

	results := make([]T, 0, minLen)
	for i := 0; i < minLen; i++ {
		rowRaw := make([]string, len(keys))
		for j := range keys {
			rowRaw[j] = strings.TrimSpace(allSlices[j][i])
		}

		results = append(results, mapper(rowRaw))
	}

	return results
}
