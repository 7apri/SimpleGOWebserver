package main

import (
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"
)

type WeatherServicePayload struct {
	addrs       *LocationReadableAdress
	weatherData *WeatherData
}

type WeatherService struct {
	*Database
	cache     *lru.Cache[uint, *WeatherData]
	sfG       singleflight.Group
	saveQueue chan *WeatherServicePayload
}

/*func (wS *WeatherService) GetWeatherData() (*WeatherData, error) {
	if data, ok := wS.cache.Get(key); ok {
		return data, nil
	}

	val, err, _ := wS.sfG.Do(key, func() (any, error) {
		weather, err := wS.DB.FindWeatherByLocationId()
		if err == nil && !weather.IsStale() {
			return weather, nil
		}

		return wS.fetchAndQueue(addrs)
	})

	if err != nil {
		return nil, err
	}

	result := val.(*WeatherData)

	if result != nil {
		wS.cache.Add(key, result)
	}

	return result, nil
}

func (wS *WeatherService) fetchAndQueue(addrs *FullAdress) (*WeatherData, error) {
	data, err := GetWeatherDataApi(addrs.Coordinates)
	if err != nil {
		return nil, err
	}

	wS.saveQueue <- &WeatherServicePayload{
		addrs:       &addrs.LocationReadableAdress,
		weatherData: data,
	}

	return data, nil
}*/

func NewWeatherService(db *Database, cacheSize int) (*WeatherService, error) {
	c, _ := lru.New[uint, *WeatherData](cacheSize)

	return &WeatherService{
		Database:  db,
		cache:     c,
		saveQueue: make(chan *WeatherServicePayload, 100),
	}, nil
}
