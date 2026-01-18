package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type GeoResult struct {
	LocationReadableAdress
	Coordinates
	LocalNames map[string]string `json:"local_names"`
}

type IpGeoResult struct {
	Status   string `json:"status"`
	Country  string `json:"countryCode"`
	State    string `json:"regionName"`
	CityName string `json:"city"`
	Coordinates
}

type LocationReadableAdress struct {
	CityName string `json:"name"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country"`
}

type Coordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func IpToCoordinates(ip string) (*IpGeoResult, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?language=en", ip)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result IpGeoResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status == "fail" {
		return nil, fmt.Errorf("ip geo failed for: %s", ip)
	}

	return &result, nil
}

func ReverseGeolocate(lat float64, lon float64) ([]GeoResult, error) {
	url := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/reverse?lat=%f&lon=%f&limit=1&appid=%s",
		lat, lon, weatherApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []GeoResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func Geolocate(adress LocationReadableAdress) ([]GeoResult, error) {
	q := adress.CityName
	if adress.State != "" {
		q += "," + adress.State
	}
	q += "," + adress.Country

	params := url.Values{}
	params.Add("q", q)
	params.Add("limit", "1")
	params.Add("appid", weatherApiKey)

	resp, err := http.Get("http://api.openweathermap.org/geo/1.0/direct?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []GeoResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}
