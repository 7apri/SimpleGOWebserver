package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	util "github.com/7apri/SimpleGOWebserver/pkg"
	"github.com/bytedance/sonic"
)

func (server *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	dbLatency, err := server.database.GetLatency()
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
		return &location.LocationResolveIn{}
	},
}

func (server *Server) HandleLocation(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx := r.Context()

	var (
		coords    []exApi.Coordinates
		addresses []exApi.LocationReadableAddress
		ips       []string
	)

	if latParam, lonParam := query.Get("lat"), query.Get("lon"); latParam != "" && lonParam != "" {
		coords = util.ParseGenericQuery(func(row []string) exApi.Coordinates {
			lt, _ := strconv.ParseFloat(row[0], 64)
			ln, _ := strconv.ParseFloat(row[1], 64)
			return exApi.Coordinates{Lat: lt, Lon: ln}
		}, latParam, lonParam)
	}

	if cityParam, countryParam := query.Get("city"), query.Get("country"); cityParam != "" && countryParam != "" {
		addresses = util.ParseGenericQuery(func(row []string) exApi.LocationReadableAddress {
			state := row[1]
			if state == "-" {
				state = ""
			}
			return exApi.LocationReadableAddress{
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
	finalData := make([]json.RawMessage, totalExpected)

	type job struct {
		index uint
		in    *location.LocationResolveIn
	}

	jobs := make(chan job, totalExpected)
	var wg sync.WaitGroup

	workerCount := min(totalExpected, 8)

	for range workerCount {
		go func() {
			for j := range jobs {
				res, jsonBytes, err := server.locationService.ResolveLocation(ctx, j.in)

				if err == nil {
					if jsonBytes == nil {
						jsonBytes, _ = sonic.Marshal(res)
					}
					finalData[j.index] = jsonBytes
				}

				resolveInPool.Put(j.in)
				wg.Done()
			}
		}()
	}

	feedJob := func(idx uint, setup func(*location.LocationResolveIn)) {
		in := resolveInPool.Get().(*location.LocationResolveIn)
		in.Reset()
		setup(in)
		wg.Add(1)
		jobs <- job{index: idx, in: in}
	}

	var currIdx uint
	for _, c := range coords {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.Coordinates = c })
		currIdx++
	}
	for _, a := range addresses {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.LocationReadableAddress = a })
		currIdx++
	}
	for _, ip := range ips {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.IP = ip })
		currIdx++
	}

	close(jobs)
	wg.Wait()

	finalData = slices.DeleteFunc(finalData, func(r json.RawMessage) bool { return r == nil })

	if len(finalData) == 0 {
		util.SendErrorJson(w, "no locations resolved", http.StatusNotFound)
		return
	}
	util.SendRawJsonSlice(w, http.StatusOK, finalData)
}

func (server *Server) HandleWeather(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	ctx := r.Context()

	var (
		coords    []exApi.Coordinates
		addresses []exApi.LocationReadableAddress
		ips       []string
	)

	if latParam, lonParam := query.Get("lat"), query.Get("lon"); latParam != "" && lonParam != "" {
		coords = util.ParseGenericQuery(func(row []string) exApi.Coordinates {
			lt, _ := strconv.ParseFloat(row[0], 64)
			ln, _ := strconv.ParseFloat(row[1], 64)
			return exApi.Coordinates{Lat: lt, Lon: ln}
		}, latParam, lonParam)
	}

	if cityParam, countryParam := query.Get("city"), query.Get("country"); cityParam != "" && countryParam != "" {
		addresses = util.ParseGenericQuery(func(row []string) exApi.LocationReadableAddress {
			state := row[1]
			if state == "-" {
				state = ""
			}
			return exApi.LocationReadableAddress{
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
	finalData := make([]json.RawMessage, totalExpected)

	type job struct {
		index uint
		in    *location.LocationResolveIn
	}

	jobs := make(chan job, totalExpected)
	var wg sync.WaitGroup

	workerCount := min(totalExpected, 8)

	for range workerCount {
		go func() {
			for j := range jobs {
				res, jsonBytes, err := server.weatherService.GetWeather(ctx, j.in)

				if err == nil {
					if jsonBytes == nil {
						jsonBytes, _ = sonic.Marshal(res)
					}
					finalData[j.index] = json.RawMessage(jsonBytes)
				}

				resolveInPool.Put(j.in)
				wg.Done()
			}
		}()
	}

	feedJob := func(idx uint, setup func(*location.LocationResolveIn)) {
		in := resolveInPool.Get().(*location.LocationResolveIn)
		in.Reset()
		setup(in)
		wg.Add(1)
		jobs <- job{index: idx, in: in}
	}

	var currIdx uint
	for _, c := range coords {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.Coordinates = c })
		currIdx++
	}
	for _, a := range addresses {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.LocationReadableAddress = a })
		currIdx++
	}
	for _, ip := range ips {
		feedJob(currIdx, func(i *location.LocationResolveIn) { i.IP = ip })
		currIdx++
	}

	close(jobs)
	wg.Wait()

	util.SendJson(w, http.StatusOK, slices.DeleteFunc(finalData, func(r json.RawMessage) bool { return r == nil }))
}
