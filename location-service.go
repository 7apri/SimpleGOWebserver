package main

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

type LocationService struct {
	*Database
	cache     *lru.Cache[string, uint]
	sfG       singleflight.Group
	saveQueue chan *GeoResult
}

type LocationResolveIn struct {
	FullAdress
	IP string `json:"ip,omitempty"`
}

func (lS *LocationService) ResolveLocation(ctx context.Context, data *LocationResolveIn) (*GeoResult, error) {
	return &GeoResult{}, nil
}

func NewLocationService(db *Database, cacheSize int) (*LocationService, error) {
	c, _ := lru.New[string, uint](cacheSize)

	return &LocationService{
		Database:  db,
		cache:     c,
		saveQueue: make(chan *GeoResult, 100),
	}, nil
}
