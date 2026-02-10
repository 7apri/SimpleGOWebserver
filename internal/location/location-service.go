package location

import (
	"context"
	"fmt"
	"hash/maphash"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/bytedance/sonic"

	"golang.org/x/sync/singleflight"
)

type LocationResolveIn struct {
	exApi.FullAddress
	IP        string `json:"ip,omitempty"`
	builder   strings.Builder
	cachedKey atomic.Pointer[string]
}

func (l *LocationResolveIn) Reset() {
	l.cachedKey.Store(nil)

	l.CityName = ""
	l.Country = ""
	l.State = ""
	l.IP = ""
	l.Lat = 0
	l.Lon = 0
}
func (lR *LocationResolveIn) Key() string {
	p := lR.cachedKey.Load()
	if p != nil {
		return *p
	}

	lR.builder.Reset()
	lR.builder.Grow(32)

	if lR.CityName != "" && lR.Country != "" {
		lR.builder.WriteString("a:")
		lR.LocationReadableAddress.WriteKey(&lR.builder)
	}
	if lR.Lat != 0 || lR.Lon != 0 {
		lR.builder.WriteString("c:")
		lR.Coordinates.WriteKey(&lR.builder)
	}
	if lR.IP != "" {
		lR.builder.WriteString("i:")
		lR.builder.WriteString(lR.IP)
	}

	finalStr := lR.builder.String()

	lR.cachedKey.Store(&finalStr)

	return finalStr
}

func (lR *LocationResolveIn) ResetKey() {
	lR.cachedKey.Store(nil)
}

type LocationService struct {
	DB        *database.Database
	cache     *cache.TieredCache[string, *exApi.GeoResult]
	sfG       singleflight.Group
	saveQueue chan *exApi.GeoResult
	owClient  *exApi.OpenWeatherClient
	ipClient  *exApi.IpApiClient
	wg        sync.WaitGroup
}

func NewService(ctx context.Context, db *database.Database, cacheSize int, owClient *exApi.OpenWeatherClient, ipClient *exApi.IpApiClient) (*LocationService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(cacheSize, 4, 20, 100,
		func(data *exApi.GeoResult) ([]byte, error) {
			return sonic.Marshal(data)
		}, func(key string) uint32 {
			return uint32(maphash.String(s, key))
		})
	service := LocationService{
		DB:        db,
		cache:     c,
		saveQueue: make(chan *exApi.GeoResult, 100),
		owClient:  owClient,
		ipClient:  ipClient,
	}
	go service.saver()

	return &service, nil
}

func (wS *LocationService) Down(ctx context.Context) error {
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

func (lS *LocationService) saver() {
	const timeout = 5 * time.Millisecond
	const batchLimit = 40

	timer := time.NewTimer(timeout)
	batch := make([]*exApi.GeoResult, 0, batchLimit)

	for {
		select {
		case report, ok := <-lS.saveQueue:
			if !ok {
				lS.flush(batch)
				break
			}
			batch = append(batch, report)
			if len(batch) >= batchLimit {
				lS.flush(batch)
				batch = batch[:0]
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(timeout)
			}
		case <-timer.C:
			if len(batch) > 0 {
				lS.flush(batch)
				batch = batch[:0]
			}
			timer.Reset(timeout)
		}
	}
}

func (lS *LocationService) ResolveLocation(ctx context.Context, locationIn *LocationResolveIn) (*exApi.GeoResult, []byte, error) {
	if data, jsonBytes, ok := lS.cache.Get(locationIn.Key()); ok {
		return data, jsonBytes, nil
	}

	if locationIn.IP != "" && locationIn.CityName == "" {
		val, err, _ := lS.sfG.Do(locationIn.IP, func() (any, error) {
			return lS.ipClient.IpToCoordinates(ctx, locationIn.IP)
		})
		if err == nil {
			ipRes := val.(*exApi.IpGeoResult)
			locationIn.LocationReadableAddress = ipRes.GetAddress()
			locationIn.Coordinates = ipRes.Coordinates

			locationIn.ResetKey()

			if data, jsonBytes, ok := lS.cache.Get(locationIn.Key()); ok {
				return data, jsonBytes, nil
			}
		}
	}

	val, err, _ := lS.sfG.Do(locationIn.Key(), func() (any, error) {
		var result *exApi.GeoResult
		var err error

		if locationIn.CityName != "" {
			result, err = lS.FindExactLocationByAddress(ctx, &locationIn.LocationReadableAddress)
			if err != nil {
				data, apiErr := lS.owClient.Geolocate(ctx, &locationIn.LocationReadableAddress)
				if apiErr == nil && len(data) > 0 {
					result = &data[0]
					lS.wg.Add(1)
					lS.saveQueue <- result
				}
			}
		} else if locationIn.Lat != 0 {
			result, err = lS.FindLocationByCoords(ctx, &locationIn.Coordinates)
			if err != nil {
				data, apiErr := lS.owClient.ReverseGeolocate(ctx, &locationIn.Coordinates)
				if apiErr == nil && len(data) > 0 {
					result = &data[0]
					lS.wg.Add(1)
					lS.saveQueue <- result
				}
			}
		}

		if result == nil {
			return nil, fmt.Errorf("location not found")
		}
		return result, nil
	})

	if err != nil {
		return nil, nil, err
	}

	finalResult := val.(*exApi.GeoResult)

	var b strings.Builder
	b.Grow(64)

	b.WriteString("a:")
	finalResult.LocationReadableAddress.WriteKey(&b)
	lS.cache.Add(b.String(), finalResult)

	b.Reset()

	b.WriteString("c:")
	finalResult.Coordinates.WriteKey(&b)
	lS.cache.Add(b.String(), finalResult)

	lS.cache.Add(locationIn.Key(), finalResult)

	return finalResult, nil, nil
}
