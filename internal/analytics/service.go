package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	db         *database.Database
	redis      *redis.Client
	LogHandler *Handler
	workerWg   sync.WaitGroup
	stopChan   chan struct{}
}

func NewService(db *database.Database, rdb *redis.Client) *Service {
	sv := &Service{
		db:       db,
		redis:    rdb,
		stopChan: make(chan struct{}),
	}

	sv.LogHandler = &Handler{
		service: sv,
		console: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}

	sv.workerWg.Add(1)
	go sv.StartWorker()

	return sv
}

func (s *Service) Down(ctx context.Context) error {
	close(s.stopChan)

	done := make(chan struct{})
	go func() {
		s.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

func (s *Service) StartWorker() {
	defer s.workerWg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			s.runFlushCycle()
			return
		case <-ticker.C:
			s.runFlushCycle()
		}
	}
}
