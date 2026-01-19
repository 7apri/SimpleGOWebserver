package main

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
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
	query := r.URL.Query()

	latParam := query.Get("lat")
	lonParam := query.Get("lon")

	if latParam != "" && lonParam != "" {
		sendJson(w, http.StatusOK, parseGenericQuery(query, func(row []string) Coordinates {
			lat, _ := strconv.ParseFloat(row[0], 64)
			lon, _ := strconv.ParseFloat(row[1], 64)
			return Coordinates{Lat: lat, Lon: lon}
		}, "lat", "lon"))
		return
	}

	countryParam := query.Get("country")
	cityParam := query.Get("city")

	if countryParam != "" && cityParam != "" {
		sendJson(w, http.StatusOK, parseGenericQuery(query, func(row []string) LocationReadableAdress {
			state := row[1]
			if state == "-" {
				state = ""
			}
			return LocationReadableAdress{
				CityName: strings.TrimSpace(row[0]),
				State:    strings.TrimSpace(state),
				Country:  strings.TrimSpace(row[2]),
			}
		}, "city", "state", "country"))
		return
	}

	if ipParam := query.Get("ip"); ipParam != "" {
		type dunno struct {
			Ip string `json:"ip"`
		}
		sendJson(w, http.StatusOK, parseGenericQuery(query, func(row []string) dunno {
			return dunno{
				Ip: row[0],
			}
		}, "ip"))
		return
	}

	sendErrorJson(w, "Valid params not found: [country&city(state)||ip]", http.StatusBadRequest)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {

}
func HandleRegister(w http.ResponseWriter, r *http.Request) {
}
