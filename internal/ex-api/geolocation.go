package exApi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"
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

func (c *IpApiClient) IpToCoordinates(ctx context.Context, ip string) (*IpGeoResult, error) {
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

	var result IpGeoResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status == "fail" {
		return nil, fmt.Errorf("ip geo failed for: %s", ip)
	}

	return &result, nil
}

func (c *OpenWeatherClient) ReverseGeolocate(ctx context.Context, coords *Coordinates) ([]GeoResult, error) {
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

	var results []GeoResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func (c *OpenWeatherClient) Geolocate(ctx context.Context, address *LocationReadableAddress) ([]GeoResult, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	slog.Warn("Geo $$")

	q := strings.Map(func(r rune) rune {
		if r == '-' {
			return ' '
		}
		return r
	}, address.Key())

	baseURL := "http://api.openweathermap.org/geo/1.0/direct"
	params := url.Values{}
	params.Add("q", q)
	params.Add("appid", c.apiKey)

	u := baseURL + "?" + params.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []GeoResult

	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

//http://localhost/api/location?city=Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague,Prague&country=CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ,CZ&state=-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-
