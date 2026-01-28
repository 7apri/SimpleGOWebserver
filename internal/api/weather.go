package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	lc "github.com/7apri/SimpleGOWebserver/internal/location"
	wt "github.com/7apri/SimpleGOWebserver/internal/weather"
	"golang.org/x/time/rate"
)

type OpenWeatherClient struct {
	apiKey  string
	HTTP    *http.Client
	limiter *rate.Limiter
}

func NewOwClient(key string, limit time.Duration) *OpenWeatherClient {
	return &OpenWeatherClient{
		apiKey:  key,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		limiter: rate.NewLimiter(rate.Every(limit), 1),
	}
}

func (c *OpenWeatherClient) GetWeatherDataApi(ctx context.Context, coords lc.Coordinates) (*wt.WeatherData, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	slog.Warn("Weather $$")

	url := fmt.Sprintf("https://api.openweathermap.org/data/3.0/onecall?lat=%f&lon=%f&appid=%s",
		coords.Lat, coords.Lon, c.apiKey)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results wt.WeatherData
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return &results, nil
}
