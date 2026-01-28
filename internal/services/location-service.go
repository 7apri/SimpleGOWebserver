package services

import (
	"context"
	"fmt"
	"hash/maphash"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/7apri/SimpleGOWebserver/internal/api"
	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/location"

	"github.com/bytedance/sonic"
	"golang.org/x/sync/singleflight"
)

type LocationService struct {
	DB        *database.Database
	cache     *cache.TieredCache[*location.GeoResult, string]
	sfG       singleflight.Group
	saveQueue chan *location.GeoResult
	owClient  *api.OpenWeatherClient
	ipClient  *api.IpApiClient
	wg        sync.WaitGroup
}

type LocationResolveIn struct {
	location.FullAddress
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
		lR.LocationReadableAddress.WriteKey(&lR.builder)
	}
	if lR.Lat != 0 || lR.Lon != 0 {
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

func (lS *LocationService) ResolveLocation(ctx context.Context, locationIn *LocationResolveIn) (*location.GeoResult, []byte, error) {
	if data, jsonBytes, ok := lS.cache.Get(locationIn.Key()); ok {
		return data, jsonBytes, nil
	}

	if locationIn.IP != "" && locationIn.CityName == "" {
		val, err, _ := lS.sfG.Do("i:"+locationIn.IP, func() (any, error) {
			return lS.ipClient.IpToCoordinates(ctx, locationIn.IP)
		})
		if err == nil {
			ipRes := val.(*location.IpGeoResult)
			locationIn.LocationReadableAddress = ipRes.GetAddress()
			locationIn.Coordinates = ipRes.Coordinates

			locationIn.ResetKey()

			if data, jsonBytes, ok := lS.cache.Get(locationIn.Key()); ok {
				return data, jsonBytes, nil
			}
		}
	}

	val, err, _ := lS.sfG.Do(locationIn.Key(), func() (any, error) {
		var result *location.GeoResult
		var err error

		if locationIn.CityName != "" {
			result, err = lS.DB.FindLocationByAddress(ctx, &locationIn.LocationReadableAddress)
			if err != nil {
				slog.Error("got an err", "err", err)
				/*data, apiErr := lS.owClient.Geolocate(ctx, &locationIn.LocationReadableAddress)
				if apiErr == nil && len(data) > 0 {
					result = &data[0]
					lS.wg.Add(1)
					lS.saveQueue <- result
				}*/
			}
		} else if locationIn.Lat != 0 {
			result, err = lS.DB.FindLocationByCoords(ctx, &locationIn.Coordinates)
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

	finalResult := val.(*location.GeoResult)

	lS.cache.Add(locationIn.Coordinates.Key(), finalResult)
	lS.cache.Add(locationIn.LocationReadableAddress.Key(), finalResult)

	return finalResult, nil, nil
}

func (lS *LocationService) locationSaver() {
	for location := range lS.saveQueue {
		lS.DB.SaveLocation(location)
		lS.wg.Done()
	}
}

func (lS *LocationService) Down() {
	close(lS.saveQueue)
	lS.wg.Wait()
}

func NewLocationService(db *database.Database, cacheSize int, apiKey string, owClient *api.OpenWeatherClient, ipClient *api.IpApiClient) (*LocationService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(cacheSize, 16, 20, 1000,
		func(data *location.GeoResult) ([]byte, error) {
			return sonic.Marshal(data)
		}, func(key string) uint32 {
			return uint32(maphash.String(s, key))
		})
	service := LocationService{
		DB:        db,
		cache:     c,
		saveQueue: make(chan *location.GeoResult, 100),
		owClient:  owClient,
		ipClient:  ipClient,
	}
	go service.locationSaver()

	return &service, nil
}
