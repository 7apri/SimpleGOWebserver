package exApi

import (
	"math"
	"time"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

type WeatherReport struct {
	Address *LocationReadableLocalizedAddress `json:"address,omitempty"`
	Data    *WeatherData                      `json:"data"`
}

type WeatherReportId struct {
	LocationId int64
	Report     *WeatherReport
}

type WeatherReportGeoRes struct {
	GeoRes *GeoResult
	Report *WeatherReport
}

type Unit uint8

const (
	UnitStandard Unit = iota // 0
	UnitMetric               // 1
	UnitImperial             // 2
)

type Exclude uint8

const (
	ExcludeMinutely Exclude = 1 << iota // 1
	ExcludeHourly                       // 2
	ExcludeDaily                        // 4
	ExcludeAlerts                       // 8
	ExcludeCurrent                      // 16
	ExcludeAddress                      // 32

)

// One Call API 3.0 response yall dont even wanna know how long it took
// Documentation: https://openweathermap.org/api/one-call-3
type WeatherData struct {
	Lat            float64    `json:"lat"`
	Lon            float64    `json:"lon"`
	Timezone       string     `json:"timezone"`
	TimezoneOffset int        `json:"timezone_offset"`
	Current        *Current   `json:"current,omitempty"`
	Hourly         []Hourly   `json:"hourly,omitempty"`
	Minutely       []Minutely `json:"minutely,omitempty"`
	Daily          []Daily    `json:"daily,omitempty"`
	Alerts         []Alert    `json:"alerts,omitempty"`
}

func (r *WeatherReport) ConvertAndFilter(unit Unit, exc Exclude) *WeatherReport {
	newReport := *r
	if exc&ExcludeAddress != 0 {
		newReport.Address = nil
	}
	newReport.Data = r.Data.Filter(exc)
	newReport.Data = newReport.Data.ConvertUnits(unit)
	return &newReport
}

func (r *WeatherReport) ConvertUnits(unit Unit) *WeatherReport {
	newReport := *r
	newReport.Data = r.Data.ConvertUnits(unit)
	return &newReport
}

func (wd *WeatherData) Filter(flags Exclude) *WeatherData {
	if flags == 0 {
		return wd
	}

	newData := *wd

	if flags&ExcludeMinutely != 0 {
		newData.Minutely = nil
	}
	if flags&ExcludeHourly != 0 {
		newData.Hourly = nil
	}
	if flags&ExcludeDaily != 0 {
		newData.Daily = nil
	}
	if flags&ExcludeAlerts != 0 {
		newData.Alerts = nil
	}
	if flags&ExcludeCurrent != 0 {
		newData.Current = nil
	}

	return &newData
}
func kToC(k float64) float64     { return math.Round((k-273.15)*10) / 10 }
func kToF(k float64) float64     { return math.Round(((k-273.15)*1.8+32)*10) / 10 }
func msToMph(ms float64) float64 { return math.Round((ms*2.23694)*100) / 100 }
func same(v float64) float64     { return v }

func (d *WeatherData) ConvertUnits(unit Unit) *WeatherData {
	if unit == UnitStandard || d == nil {
		return d
	}

	var tConv func(float64) float64
	if unit == UnitMetric {
		tConv = kToC
	} else {
		tConv = kToF
	}

	var sConv func(float64) float64 = same
	if unit == UnitImperial {
		sConv = msToMph
	}

	convPtr := func(k *float64, fn func(float64) float64) *float64 {
		if k == nil {
			return nil
		}
		v := fn(*k)
		return &v
	}

	newData := *d

	if newData.Current != nil {
		c := *newData.Current
		c.Temp = tConv(c.Temp)
		c.FeelsLike = convPtr(c.FeelsLike, tConv)
		c.DewPoint = convPtr(c.DewPoint, tConv)
		c.WindSpeed = sConv(c.WindSpeed)
		c.WindGust = convPtr(c.WindGust, sConv)
		newData.Current = &c
	}

	if len(d.Hourly) > 0 {
		newHourly := make([]Hourly, len(d.Hourly))
		for i, h := range d.Hourly {
			h.Temp = tConv(h.Temp)
			h.FeelsLike = convPtr(h.FeelsLike, tConv)
			h.DewPoint = convPtr(h.DewPoint, tConv)
			h.WindSpeed = sConv(h.WindSpeed)
			h.WindGust = convPtr(h.WindGust, sConv)
			newHourly[i] = h
		}
		newData.Hourly = newHourly
	}

	if len(d.Daily) > 0 {
		newDaily := make([]Daily, len(d.Daily))
		for i, daily := range d.Daily {
			daily.Temp.Day = tConv(daily.Temp.Day)
			daily.Temp.Min = tConv(daily.Temp.Min)
			daily.Temp.Max = tConv(daily.Temp.Max)
			daily.Temp.Night = tConv(daily.Temp.Night)
			daily.Temp.Evening = tConv(daily.Temp.Evening)
			daily.Temp.Morning = tConv(daily.Temp.Morning)

			if daily.FeelsLike != nil {
				fl := *daily.FeelsLike
				fl.Day = tConv(fl.Day)
				fl.Night = tConv(fl.Night)
				fl.Evening = tConv(fl.Evening)
				fl.Morning = tConv(fl.Morning)
				daily.FeelsLike = &fl
			}

			daily.DewPoint = convPtr(daily.DewPoint, tConv)
			daily.WindSpeed = sConv(daily.WindSpeed)
			daily.WindGust = convPtr(daily.WindGust, sConv)
			newDaily[i] = daily
		}
		newData.Daily = newDaily
	}

	return &newData
}

func (data *WeatherData) IsFresh() bool {
	if data == nil || data.Current == nil {
		return false
	}
	return time.Since(time.Unix(data.Current.Dt, 0)) < 10*time.Minute
}

func (wd *WeatherData) Localize(trMap util.ReadOnlyMap[int16, string]) *WeatherData {
	if wd == nil || trMap.Len() == 0 {
		return wd
	}

	newData := *wd

	translateSlice := func(descs []WeatherDesc) []WeatherDesc {
		if len(descs) == 0 {
			return nil
		}
		newDescs := make([]WeatherDesc, len(descs))
		copy(newDescs, descs)

		for i := range newDescs {
			if translated, ok := trMap.Lookup(newDescs[i].ID); ok {
				newDescs[i].Description = translated
			}
		}
		return newDescs
	}

	if wd.Current != nil {
		newCurrent := *wd.Current
		newCurrent.Weather = translateSlice(wd.Current.Weather)
		newData.Current = &newCurrent
	}
	if len(newData.Daily) > 0 {
		newDaily := make([]Daily, len(newData.Daily))
		copy(newDaily, newData.Daily)
		for i := range newDaily {
			newDaily[i].Weather = translateSlice(newDaily[i].Weather)
		}
		newData.Daily = newDaily
	}

	if len(newData.Hourly) > 0 {
		newHourly := make([]Hourly, len(newData.Hourly))
		copy(newHourly, newData.Hourly)
		for i := range newHourly {
			newHourly[i].Weather = translateSlice(newHourly[i].Weather)
		}
		newData.Hourly = newHourly
	}

	return &newData
}

func (wd *WeatherData) ToReport(address *LocationReadableLocalizedAddress) *WeatherReport {
	return &WeatherReport{
		Address: address,
		Data:    wd,
	}
}
func (wd *WeatherData) ToReportGeoRes(geoRes *GeoResult) WeatherReportGeoRes {
	return WeatherReportGeoRes{
		GeoRes: geoRes,
		Report: wd.ToReport(&geoRes.LocationReadableLocalizedAddress),
	}
}
func (wd *WeatherData) ToReportId(id int64, address *LocationReadableLocalizedAddress) WeatherReportId {
	return WeatherReportId{
		LocationId: id,
		Report:     wd.ToReport(address),
	}
}

type WeatherDesc struct {
	ID          int16  `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type Current struct {
	Dt         int64          `json:"dt"`
	Sunrise    *int64         `json:"sunrise,omitempty"`
	Sunset     *int64         `json:"sunset,omitempty"`
	Temp       float64        `json:"temp"`
	FeelsLike  *float64       `json:"feels_like,omitempty"`
	Pressure   int            `json:"pressure"`
	Humidity   int            `json:"humidity"`
	DewPoint   *float64       `json:"dew_point,omitempty"`
	Uvi        *float64       `json:"uvi,omitempty"`
	Clouds     int            `json:"clouds"`
	Visibility int            `json:"visibility"`
	WindSpeed  float64        `json:"wind_speed"`
	WindGust   *float64       `json:"wind_gust,omitempty"`
	WindDeg    int            `json:"wind_deg"`
	Rain       *Precipitation `json:"rain,omitempty"`
	Snow       *Precipitation `json:"snow,omitempty"`
	Weather    []WeatherDesc  `json:"weather"`
}

type Hourly struct {
	Dt         int64          `json:"dt"`
	Temp       float64        `json:"temp"`
	FeelsLike  *float64       `json:"feels_like,omitempty"`
	Pressure   int            `json:"pressure"`
	Humidity   int            `json:"humidity"`
	DewPoint   *float64       `json:"dew_point,omitempty"`
	Uvi        *float64       `json:"uvi,omitempty"`
	Clouds     int            `json:"clouds"`
	Visibility int            `json:"visibility"`
	WindSpeed  float64        `json:"wind_speed"`
	WindGust   *float64       `json:"wind_gust,omitempty"`
	WindDeg    int            `json:"wind_deg"`
	Pop        float64        `json:"pop"`
	Rain       *Precipitation `json:"rain,omitempty"`
	Snow       *Precipitation `json:"snow,omitempty"`
	Weather    []WeatherDesc  `json:"weather"`
}

type Daily struct {
	Dt        int64           `json:"dt"`
	Sunrise   *int64          `json:"sunrise,omitempty"`
	Sunset    *int64          `json:"sunset,omitempty"`
	Moonrise  *int64          `json:"moonrise,omitempty"`
	Moonset   *int64          `json:"moonset,omitempty"`
	MoonPhase float64         `json:"moon_phase"`
	Summary   string          `json:"summary,omitempty"`
	Temp      DailyTemp       `json:"temp"`
	FeelsLike *DailyFeelsLike `json:"feels_like,omitempty"`
	Pressure  int             `json:"pressure"`
	Humidity  int             `json:"humidity"`
	DewPoint  *float64        `json:"dew_point,omitempty"`
	WindSpeed float64         `json:"wind_speed"`
	WindGust  *float64        `json:"wind_gust,omitempty"`
	WindDeg   int             `json:"wind_deg"`
	Clouds    int             `json:"clouds"`
	Uvi       *float64        `json:"uvi,omitempty"`
	Pop       float64         `json:"pop"`
	Rain      *float64        `json:"rain,omitempty"`
	Snow      *float64        `json:"snow,omitempty"`
	Weather   []WeatherDesc   `json:"weather"`
}

type DailyTemp struct {
	Morning float64 `json:"morn"`
	Day     float64 `json:"day"`
	Evening float64 `json:"eve"`
	Night   float64 `json:"night"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

type DailyFeelsLike struct {
	Morning float64 `json:"morn"`
	Day     float64 `json:"day"`
	Evening float64 `json:"eve"`
	Night   float64 `json:"night"`
}

type Minutely struct {
	Dt            int64   `json:"dt"`
	Precipitation float64 `json:"precipitation"`
}

type Precipitation struct {
	OneH float64 `json:"1h"`
}

type Alert struct {
	SenderName  string   `json:"sender_name"`
	Event       string   `json:"event"`
	Start       int64    `json:"start"`
	End         int64    `json:"end"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
