package location

import (
	"context"
	"fmt"
	"hash/maphash"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/cache"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/bytedance/sonic"
	"golang.org/x/sync/singleflight"
)

type LocationService struct {
	DB        *database.Database
	cache     *cache.TieredCache[string, *exApi.GeoResult]
	sfG       singleflight.Group
	saveQueue chan *exApi.GeoResult
	owClient  *exApi.OpenWeatherClient
	ipClient  *exApi.IpApiClient
	wg        sync.WaitGroup
}

func NewService(db *database.Database, cacheSize int, promoteThreshold int64, promotioChanBufferSize int, janitorInterval time.Duration, saveChanBufferSize int, owClient *exApi.OpenWeatherClient, ipClient *exApi.IpApiClient) (*LocationService, error) {
	s := maphash.MakeSeed()
	c := cache.NewTieredCache(
		cacheSize,
		promoteThreshold,
		promotioChanBufferSize,
		janitorInterval,
		func(data *exApi.GeoResult) ([]byte, error) {
			return sonic.Marshal(data)
		},
		func(key string) uint32 {
			return uint32(maphash.String(s, key))
		},
	)
	service := LocationService{
		DB:        db,
		cache:     c,
		saveQueue: make(chan *exApi.GeoResult, saveChanBufferSize),
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
