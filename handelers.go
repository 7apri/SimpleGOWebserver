package main

import (
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
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

func (server *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	dbLatency, err := server.database.GetLatency()
	if err != nil {
		http.Error(w, "Database unreachable", http.StatusInternalServerError)
		return
	}

	googleLatency, err := PingGoogle()
	if err != nil {
		http.Error(w, "Internet unreachable", http.StatusInternalServerError)
		return
	}

	SendJson(w, http.StatusOK, struct {
		Status string `json:"status"`
		DbPing string `json:"dbPing"`
		ExPing string `json:"exPing"`
	}{
		Status: "Healthy",
		DbPing: dbLatency,
		ExPing: googleLatency,
	})
}

func (server *Server) HandleLocation(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx := r.Context()

	var (
		coords   []Coordinates
		adresses []LocationReadableAdress
		ips      []string
	)

	if latParam, lonParam := query.Get("lat"), query.Get("lon"); latParam != "" && lonParam != "" {
		coords = ParseGenericQuery(func(row []string) Coordinates {
			lt, _ := strconv.ParseFloat(row[0], 64)
			ln, _ := strconv.ParseFloat(row[1], 64)
			return Coordinates{Lat: lt, Lon: ln}
		}, latParam, lonParam)
	}

	if cityParam, countryParam := query.Get("city"), query.Get("country"); cityParam != "" && countryParam != "" {
		adresses = ParseGenericQuery(func(row []string) LocationReadableAdress {
			state := row[1]
			if state == "-" {
				state = ""
			}
			return LocationReadableAdress{
				CityName: strings.TrimSpace(row[0]),
				State:    strings.TrimSpace(state),
				Country:  strings.TrimSpace(row[2]),
			}
		}, cityParam, query.Get("state"), countryParam)
	}

	if ipParam := query.Get("ip"); ipParam != "" {
		ips = strings.Split(ipParam, ",")
	}

	totalExpected := len(coords) + len(adresses) + len(ips)
	finalData := make([]*GeoResult, 0, totalExpected)

	var wg sync.WaitGroup

	var currentIndex uint = 0

	for _, coordinate := range coords {
		wg.Add(1)
		go func(c Coordinates, index uint) {
			defer wg.Done()
			locationData, err :=
				server.locationService.ResolveLocation(
					ctx,
					&LocationResolveIn{
						FullAdress: FullAdress{
							Coordinates: c,
						},
					})
			if err != nil {
				return
			}
			finalData[index] = locationData
		}(coordinate, currentIndex)
		currentIndex++
	}

	for _, adress := range adresses {
		wg.Add(1)
		go func(a LocationReadableAdress, index uint) {
			defer wg.Done()
			locationData, err :=
				server.locationService.ResolveLocation(
					ctx,
					&LocationResolveIn{
						FullAdress: FullAdress{
							LocationReadableAdress: a,
						},
					})
			if err != nil {
				return
			}
			finalData[index] = locationData
		}(adress, currentIndex)
		currentIndex++
	}

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string, index uint) {
			defer wg.Done()
			locationData, err :=
				server.locationService.ResolveLocation(
					ctx,
					&LocationResolveIn{
						IP: ip,
					},
				)
			if err != nil {
				return
			}
			finalData[index] = locationData
		}(ip, currentIndex)
		currentIndex++
	}

	wg.Wait()
	SendJson(w, http.StatusOK, slices.DeleteFunc(finalData, func(r *GeoResult) bool { return r == nil }))
}

func (server *Server) HandleWeather(w http.ResponseWriter, r *http.Request) {
	SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}

func (server *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}
func (server *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}
