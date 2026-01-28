package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	lc "github.com/7apri/SimpleGOWebserver/internal/location"
	"golang.org/x/time/rate"
)

type IpApiClient struct {
	HTTP    *http.Client
	limiter *rate.Limiter
}

func NewIpClient(limit time.Duration) *IpApiClient {
	return &IpApiClient{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		limiter: rate.NewLimiter(rate.Every(limit), 1),
	}
}

func (c *IpApiClient) IpToCoordinates(ctx context.Context, ip string) (*lc.IpGeoResult, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	slog.Warn("Ip $$")

	u := fmt.Sprintf("http://ip-api.com/json/%s?language=en", ip)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result lc.IpGeoResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status == "fail" {
		return nil, fmt.Errorf("ip geo failed for: %s", ip)
	}

	return &result, nil
}

func (c *OpenWeatherClient) ReverseGeolocate(ctx context.Context, coords *lc.Coordinates) ([]lc.GeoResult, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	slog.Warn("Rev $$")

	url := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/reverse?lat=%f&lon=%f&appid=%s",
		coords.Lat,
		coords.Lon,
		c.apiKey,
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []lc.GeoResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func (c *OpenWeatherClient) Geolocate(ctx context.Context, adress *lc.LocationReadableAddress) ([]lc.GeoResult, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	slog.Warn("Geo $$")

	u := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s",
		url.QueryEscape(adress.Key()),
		c.apiKey,
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []lc.GeoResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

//http://localhost/api/location?city=Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague&country=CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ&state=-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-
