package main

import (
	"encoding/json"
	"net/http"
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

func SendJson(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		SendErrorJson(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

type APIError struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func SendErrorJson(w http.ResponseWriter, message string, code int) {
	SendJson(w, code, APIError{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

func ParseGenericQuery[T any](mapper func([]string) T, queries ...string) []T {
	if len(queries) == 0 {
		return nil
	}

	allSlices := make([][]string, len(queries))
	minLen := -1

	for i, query := range queries {
		if query == "" {
			return nil
		}

		parts := strings.Split(query, ",")
		allSlices[i] = parts

		if minLen == -1 || len(parts) < minLen {
			minLen = len(parts)
		}
	}

	results := make([]T, 0, minLen)
	for i := 0; i < minLen; i++ {
		rowRaw := make([]string, len(queries))
		for j := range queries {
			rowRaw[j] = strings.TrimSpace(allSlices[j][i])
		}

		results = append(results, mapper(rowRaw))
	}

	return results
}

func filterNil[T any](data []*T) []*T {
	n := 0
	for _, x := range data {
		if x != nil {
			data[n] = x
			n++
		}
	}
	return data[:n]
}
