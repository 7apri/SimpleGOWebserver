package weather

import (
	"github.com/7apri/SimpleGOWebserver/internal/location"
)

type WeatherReport struct {
	Address *location.LocationReadableAddress `json:"address"`
	Data    *WeatherData                      `json:"data"`
}

type WeatherReportId struct {
	LocationId int64
	Report     *WeatherReport
}
type WeatherData struct {
	Lat            float64    `json:"lat"`
	Lon            float64    `json:"lon"`
	Timezone       string     `json:"timezone"`
	TimezoneOffset int        `json:"timezone_offset"`
	Current        Current    `json:"current"`
	Hourly         []Hourly   `json:"hourly"`
	Minutely       []Minutely `json:"minutely,omitempty"`
	Daily          []Daily    `json:"daily"`
}

func (wd *WeatherData) ToReport(address *location.LocationReadableAddress) *WeatherReport {
	return &WeatherReport{
		Address: address,
		Data:    wd,
	}
}

func (wd *WeatherData) ToReportId(id int64, address *location.LocationReadableAddress) *WeatherReportId {
	return &WeatherReportId{
		LocationId: id,
		Report:     wd.ToReport(address),
	}
}

type WeatherDesc struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type Current struct {
	Dt        int64         `json:"dt"`
	Sunrise   int64         `json:"sunrise"`
	Sunset    int64         `json:"sunset"`
	Temp      float64       `json:"temp"`
	FeelsLike float64       `json:"feels_like"`
	Pressure  int           `json:"pressure"`
	Humidity  int           `json:"humidity"`
	Uvi       float64       `json:"uvi"`
	WindSpeed float64       `json:"wind_speed"`
	Weather   []WeatherDesc `json:"weather"`
}

type Hourly struct {
	Dt       int64         `json:"dt"`
	Temp     float64       `json:"temp"`
	Pressure int           `json:"pressure"`
	Humidity int           `json:"humidity"`
	Pop      float64       `json:"pop"`
	Rain     *Rain         `json:"rain,omitempty"`
	Weather  []WeatherDesc `json:"weather"`
}

type Daily struct {
	Dt      int64         `json:"dt"`
	Summary string        `json:"summary"`
	Temp    DailyTemp     `json:"temp"`
	Weather []WeatherDesc `json:"weather"`
	Pop     float64       `json:"pop"`
	Rain    float64       `json:"rain,omitempty"`
}

type DailyTemp struct {
	Day   float64 `json:"day"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Night float64 `json:"night"`
}

type Minutely struct {
	Dt            int64   `json:"dt"`
	Precipitation float64 `json:"precipitation"`
}

type Rain struct {
	OneH float64 `json:"1h"`
}
