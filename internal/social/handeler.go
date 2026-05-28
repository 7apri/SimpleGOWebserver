package social

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type SocialWrapper struct {
	pool   *pgxpool.Pool
	redis  *redis.Client
	wg     sync.WaitGroup
	doneCh chan struct{}
}

func NewSocialWrapper(pool *pgxpool.Pool, redisClient *redis.Client) *SocialWrapper {
	return &SocialWrapper{
		pool:  pool,
		redis: redisClient,
	}
}

func (s *SocialWrapper) Start() {
	s.doneCh = make(chan struct{})
	s.wg.Add(1)
	go s.worker()
}

func (s *SocialWrapper) Down(ctx context.Context) error {
	close(s.doneCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("[Social] worker shutdown timed out: %w", ctx.Err())
	}
}

func (s *SocialWrapper) worker() {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runFlush()
		case <-s.doneCh:
			slog.Info("[Social] Shutdown signal received, performing final flush...")
			s.runFlush()
			return
		}
	}
}
