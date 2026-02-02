package services

import (
	"context"
	"hash/maphash"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/7apri/SimpleGOWebserver/internal/api"
	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	"github.com/bytedance/sonic"
)

type WeatherService struct {
	DB              *database.Database
	cache           *cache.TieredCache[*weather.WeatherReport, string]
	sfG             singleflight.Group
	saveQueue       chan *weather.WeatherReportId
	owClient        *api.OpenWeatherClient
	locationService *LocationService
	wg              sync.WaitGroup
}

func (wS *WeatherService) GetWeather(ctx context.Context, locationIn *LocationResolveIn) (*weather.WeatherReport, []byte, error) {
	if data, jsonBytes, ok := wS.cache.Get(locationIn.Key()); ok {
		return data, jsonBytes, nil
	}

	val, err, _ := wS.sfG.Do(locationIn.Key(), func() (any, error) {
		location, _, err := wS.locationService.ResolveLocation(ctx, locationIn)
		if err != nil {
			return nil, err
		}
		return location, nil
	})

	if err != nil {
		return nil, nil, err
	}

	location := val.(*location.GeoResult)
	locationId := location.GetId()
	idKey := "i:" + strconv.FormatInt(locationId, 10)

	if data, jsonBytes, ok := wS.cache.Get(idKey); ok {
		return data, jsonBytes, nil
	}

	type sfResult struct {
		report *weather.WeatherReport
		raw    []byte
	}

	weatherVal, err, _ := wS.sfG.Do(idKey, func() (any, error) {
		wd, raw, err := wS.DB.FindWeatherCacheByLocId(ctx, locationId, 9999)
		if err == nil && wd != nil {
			return &sfResult{
				report: wd,
				raw:    raw,
			}, nil
		}

		apiData, err := wS.owClient.GetWeatherDataApi(ctx, location.Coordinates)
		if err != nil {
			return nil, err
		}

		final := apiData.ToReportId(locationId, &location.LocationReadableAddress)
		wS.wg.Add(1)
		wS.saveQueue <- final

		return &sfResult{report: final.Report, raw: nil}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	result := weatherVal.(*sfResult)

	wS.cache.Add(idKey, result.report)

	var b strings.Builder
	b.Grow(64)

	b.WriteString("a:")
	location.LocationReadableAddress.WriteKey(&b)
	wS.cache.Add(b.String(), result.report)

	b.Reset()

	b.WriteString("c:")
	location.Coordinates.WriteKey(&b)
	wS.cache.Add(b.String(), result.report)

	return result.report, result.raw, nil
}

func (wS *WeatherService) weatherSaver() {
	for weatherRp := range wS.saveQueue {
		wS.DB.SaveWeatherCache(weatherRp)
		wS.wg.Done()
	}
}

func (wS *WeatherService) Down() {
	close(wS.saveQueue)
	wS.wg.Wait()
}
func NewWeatherService(db *database.Database, cacheSize int, owClient *api.OpenWeatherClient, lcService *LocationService) (*WeatherService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(cacheSize, 16, 20, 1000,
		func(data *weather.WeatherReport) ([]byte, error) {
			return sonic.Marshal(data)
		}, func(key string) uint32 {
			return uint32(maphash.String(s, key))
		})
	service := WeatherService{
		DB:              db,
		cache:           c,
		saveQueue:       make(chan *weather.WeatherReportId, 100),
		owClient:        owClient,
		locationService: lcService,
	}
	go service.weatherSaver()

	return &service, nil
}
