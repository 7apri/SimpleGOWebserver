package main

import (
	"net/http"
)

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		templates.ExecuteTemplate(w, "404.html", nil)
		return
	}

	err := templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	dbLatency, err := GetLatency()
	if err != nil {
		http.Error(w, "Database unreachable", http.StatusInternalServerError)
		return
	}

	googleLatency, err := PingGoogle()
	if err != nil {
		http.Error(w, "Internet unreachable", http.StatusInternalServerError)
		return
	}

	sendJson(w, http.StatusAccepted, struct {
		Status string `json:"status"`
		DbPing string `json:"dbPing"`
		ExPing string `json:"exPing"`
	}{
		Status: "Healthy",
		DbPing: dbLatency,
		ExPing: googleLatency,
	})
}
