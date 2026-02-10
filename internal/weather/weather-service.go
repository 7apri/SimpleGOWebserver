package weather

import (
	"context"
	"fmt"
	"hash/maphash"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/bytedance/sonic"
)

type WeatherService struct {
	DB              *database.Database
	cache           *cache.TieredCache[string, *exApi.WeatherReport]
	sfG             singleflight.Group
	saveQueue       chan exApi.WeatherReportId
	owClient        *exApi.OpenWeatherClient
	locationService *location.LocationService
	wg              sync.WaitGroup
}

func NewService(ctx context.Context, db *database.Database, cacheSize int, owClient *exApi.OpenWeatherClient, lcService *location.LocationService) (*WeatherService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(cacheSize, 8, 20, 1000,
		func(data *exApi.WeatherReport) ([]byte, error) {
			return sonic.Marshal(data)
		}, func(key string) uint32 {
			return uint32(maphash.String(s, key))
		})
	service := WeatherService{
		DB:              db,
		cache:           c,
		saveQueue:       make(chan exApi.WeatherReportId, 100),
		owClient:        owClient,
		locationService: lcService,
	}
	go service.saver()

	return &service, nil
}

func (wS *WeatherService) Down(ctx context.Context) error {
	close(wS.saveQueue)

	done := make(chan struct{})

	go func() {
		wS.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
	return nil
}

func (wS *WeatherService) saver() {
	const timeout = 5 * time.Millisecond
	const batchLimit = 40

	timer := time.NewTimer(timeout)
	batch := make([]exApi.WeatherReportId, 0, batchLimit)

	for {
		select {
		case report, ok := <-wS.saveQueue:
			if !ok {
				wS.flush(batch)
				break
			}
			batch = append(batch, report)
			if len(batch) >= batchLimit {
				wS.flush(batch)
				batch = batch[:0]
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(timeout)
			}
		case <-timer.C:
			if len(batch) > 0 {
				wS.flush(batch)
				batch = batch[:0]
			}
			timer.Reset(timeout)
		}
	}
}

func (wS *WeatherService) GetWeather(ctx context.Context, locationIn *location.LocationResolveIn) (*exApi.WeatherReport, []byte, error) {
	if report, jsonBytes, ok := wS.cache.Get(locationIn.Key()); ok {
		if time.Since(time.Unix(report.Data.Current.Dt, 0)) < 10*time.Minute {
			return report, jsonBytes, nil
		}
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

	location := val.(*exApi.GeoResult)
	locationId := location.GetId()
	if locationId != 0 {

	}
	idKey := "i:" + strconv.FormatInt(locationId, 10)

	if report, jsonBytes, ok := wS.cache.Get(idKey); ok {
		if time.Since(time.Unix(report.Data.Current.Dt, 0)) < 10*time.Minute {
			return report, jsonBytes, nil
		}
	}

	type sfResult struct {
		report *exApi.WeatherReport
		raw    []byte
	}

	weatherVal, err, _ := wS.sfG.Do(idKey, func() (any, error) {
		wd, raw, err := wS.FindWeatherCacheByLocId(ctx, locationId, 10)
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

		final := apiData.ToReportId(locationId, &location.LocationReadableLocalizedAddress)
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

	wS.cache.Add(locationIn.Key(), result.report)

	return result.report, result.raw, nil
}
