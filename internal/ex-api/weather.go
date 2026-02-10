package exApi

type WeatherReport struct {
	Address *LocationReadableLocalizedAddress `json:"address"`
	Data    *WeatherData                      `json:"data"`
}

type WeatherReportId struct {
	LocationId int64
	Report     *WeatherReport
}

// One Call API 3.0 response yall dont even wanna know how long it took
// Documentation: https://openweathermap.org/api/one-call-3
type WeatherData struct {
	Lat            float64    `json:"lat"`
	Lon            float64    `json:"lon"`
	Timezone       string     `json:"timezone"`
	TimezoneOffset int        `json:"timezone_offset"`
	Current        Current    `json:"current"`
	Hourly         []Hourly   `json:"hourly,omitempty"`
	Minutely       []Minutely `json:"minutely,omitempty"`
	Daily          []Daily    `json:"daily"`
	Alerts         []Alert    `json:"alerts,omitempty"`
}

func (wd *WeatherData) ToReport(address *LocationReadableLocalizedAddress) *WeatherReport {
	return &WeatherReport{
		Address: address,
		Data:    wd,
	}
}

func (wd *WeatherData) ToReportId(id int64, address *LocationReadableLocalizedAddress) WeatherReportId {
	return WeatherReportId{
		LocationId: id,
		Report:     wd.ToReport(address),
	}
}

type WeatherDesc struct {
	ID          uint16 `json:"id"`
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
