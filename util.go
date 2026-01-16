package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func sendJson(w http.ResponseWriter, code uint16, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Failed to marshal json payload: %v", payload)
		w.WriteHeader(500)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(code))
	w.Write(data)
}
