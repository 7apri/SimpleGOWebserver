package weather

import (
	"context"
	"fmt"
	"hash/maphash"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

type WeatherService struct {
	DB              *database.Database
	cache           *cache.TieredCache[string, *exApi.WeatherReport]
	i18n            *i18n.Manager
	sfG             singleflight.Group
	saveQueue       chan exApi.WeatherReportGeoRes
	owClient        *exApi.OpenWeatherClient
	locationService *location.LocationService
	wg              sync.WaitGroup
}

func NewService(db *database.Database, cacheSize int, promoteThreshold int64, promotioChanBufferSize int, janitorInterval time.Duration, saveChanBufferSize int, i18n *i18n.Manager, owClient *exApi.OpenWeatherClient, lcService *location.LocationService) (*WeatherService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(
		cacheSize,
		promoteThreshold,
		promotioChanBufferSize,
		time.Hour*2,
		janitorInterval,
		func(data *exApi.WeatherReport) ([]byte, error) {
			return sonic.Marshal(data)
		},
		func(key string) uint32 {
			return uint32(maphash.String(s, key))
		},
	)
	service := WeatherService{
		DB:              db,
		cache:           c,
		i18n:            i18n,
		saveQueue:       make(chan exApi.WeatherReportGeoRes, saveChanBufferSize),
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
	const batchLimit = 100

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	batch := make([]exApi.WeatherReportGeoRes, 0, batchLimit)

	b := &pgx.Batch{}

	for {
		select {
		case report, ok := <-wS.saveQueue:
			if !ok {
				if len(batch) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					wS.flush(ctx, batch, b)
					cancel()
				}
				return
			}
			batch = append(batch, report)
			if len(batch) >= batchLimit {
				wS.flush(context.Background(), batch, b)
				batch = batch[:0]
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeout)
			}
		case <-timer.C:
			if len(batch) > 0 {
				wS.flush(context.Background(), batch, b)
				batch = batch[:0]
			}
			timer.Reset(timeout)
		}
	}
}
