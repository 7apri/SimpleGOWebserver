package main

import (
	"html/template"
	"net/http"
)

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		templates, _ = template.ParseFiles("templates/404.html")
		templates.ExecuteTemplate(w, "404.html", nil)
		return
	}

	templates, _ = template.ParseFiles("templates/index.html")
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

	sendJson(w, http.StatusOK, struct {
		Status string `json:"status"`
		DbPing string `json:"dbPing"`
		ExPing string `json:"exPing"`
	}{
		Status: "Healthy",
		DbPing: dbLatency,
		ExPing: googleLatency,
	})
}

func HandleWeather(w http.ResponseWriter, r *http.Request) {
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {

}
func HandleRegister(w http.ResponseWriter, r *http.Request) {
}
