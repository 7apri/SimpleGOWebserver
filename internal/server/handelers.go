package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/services"
	util "github.com/7apri/SimpleGOWebserver/pkg"
)

type Server struct {
	LocationService *services.LocationService
	WeatherService  *services.WeatherService
	Database        *database.Database
	Templates       *template.Template
}

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		server.Templates, _ = template.ParseFiles("templates/404.html")
		server.Templates.ExecuteTemplate(w, "404.html", nil)
		return
	}

	server.Templates, _ = template.ParseFiles("templates/index.html")
	err := server.Templates.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	dbLatency, err := server.Database.GetLatency()
	if err != nil {
		http.Error(w, "Database unreachable", http.StatusInternalServerError)
		return
	}

	googleLatency, err := util.PingGoogle()
	if err != nil {
		http.Error(w, "Internet unreachable", http.StatusInternalServerError)
		return
	}

	util.SendJson(w, http.StatusOK, struct {
		Status string `json:"status"`
		DbPing string `json:"dbPing"`
		ExPing string `json:"exPing"`
	}{
		Status: "Healthy",
		DbPing: dbLatency,
		ExPing: googleLatency,
	})
}

var resolveInPool = sync.Pool{
	New: func() any {
		return &services.LocationResolveIn{}
	},
}

func (server *Server) HandleLocation(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx := r.Context()

	var (
		coords    []location.Coordinates
		addresses []location.LocationReadableAddress
		ips       []string
	)

	if latParam, lonParam := query.Get("lat"), query.Get("lon"); latParam != "" && lonParam != "" {
		coords = util.ParseGenericQuery(func(row []string) location.Coordinates {
			lt, _ := strconv.ParseFloat(row[0], 64)
			ln, _ := strconv.ParseFloat(row[1], 64)
			return location.Coordinates{Lat: lt, Lon: ln}
		}, latParam, lonParam)
	}

	if cityParam, countryParam := query.Get("city"), query.Get("country"); cityParam != "" && countryParam != "" {
		addresses = util.ParseGenericQuery(func(row []string) location.LocationReadableAddress {
			state := row[1]
			if state == "-" {
				state = ""
			}
			return location.LocationReadableAddress{
				CityName: util.CleanQuery(row[0]),
				State:    util.CleanQuery(state),
				Country:  strings.ToUpper(strings.TrimSpace(row[2])),
			}
		}, cityParam, query.Get("state"), countryParam)
	}

	if ipParam := query.Get("ip"); ipParam != "" {
		ips = strings.Split(ipParam, ",")
	}

	totalExpected := len(coords) + len(addresses) + len(ips)
	finalData := make([]any, totalExpected)

	type job struct {
		index uint
		in    *services.LocationResolveIn
	}

	jobs := make(chan job, totalExpected)
	var wg sync.WaitGroup

	workerCount := min(totalExpected, 8)

	for range workerCount {
		go func() {
			for j := range jobs {
				res, jsonBytes, err := server.LocationService.ResolveLocation(ctx, j.in)

				if err == nil {
					if jsonBytes != nil {
						finalData[j.index] = json.RawMessage(jsonBytes)
					} else {
						finalData[j.index] = res
					}
				}

				resolveInPool.Put(j.in)
				wg.Done()
			}
		}()
	}

	feedJob := func(idx uint, setup func(*services.LocationResolveIn)) {
		in := resolveInPool.Get().(*services.LocationResolveIn)
		in.Reset()
		setup(in)
		wg.Add(1)
		jobs <- job{index: idx, in: in}
	}

	var currIdx uint
	for _, c := range coords {
		feedJob(currIdx, func(i *services.LocationResolveIn) { i.Coordinates = c })
		currIdx++
	}
	for _, a := range addresses {
		feedJob(currIdx, func(i *services.LocationResolveIn) { i.LocationReadableAddress = a })
		currIdx++
	}
	for _, ip := range ips {
		feedJob(currIdx, func(i *services.LocationResolveIn) { i.IP = ip })
		currIdx++
	}

	close(jobs)
	wg.Wait()

	util.SendJson(w, http.StatusOK, slices.DeleteFunc(finalData, func(r any) bool { return r == nil }))
}

func (server *Server) HandleWeather(w http.ResponseWriter, r *http.Request) {
	util.SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}

func (server *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	util.SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}
func (server *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	util.SendErrorJson(w, "Not implemented yet", http.StatusNotImplemented)
}
