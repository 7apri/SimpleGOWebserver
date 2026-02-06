package weather

import (
	"context"
	"errors"
	"log/slog"
	"time"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

func (wS *WeatherService) SaveWeatherCache(weather *exApi.WeatherReportId) error {
	slog.Info("db save cache")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const query = `
        INSERT INTO weather_current_cache (location_id, full_data, updated_at)
        VALUES ($1, $2, NOW())
		ON CONFLICT (location_id) 
		DO UPDATE SET 
			full_data = EXCLUDED.full_data,
			updated_at = NOW();`

	_, err := wS.DB.Pool.Exec(ctx, query, weather.LocationId, weather.Report)
	return err
}
func (wS *WeatherService) SaveWeatherHistory(ctx context.Context, wr *exApi.WeatherReportId) error {
	slog.Info("db save history")
	const query = `
    INSERT INTO weather_history (
        location_id,          -- $1
        temp_avg,             -- $2
        temp_avg_feel,        -- $3
        uvi_avg,              -- $4
        pressure_avg,         -- $5
        wind_speed_avg,       -- $6
        weather_description,  -- $7
        raw_data,             -- $8
        recorded_date,        
        update_count         
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_DATE, 1)
    ON CONFLICT (location_id, recorded_date) 
    DO UPDATE SET
        temp_avg = (weather_history.temp_avg * weather_history.update_count + EXCLUDED.temp_avg) / (weather_history.update_count + 1),
        temp_avg_feel = (weather_history.temp_avg_feel * weather_history.update_count + EXCLUDED.temp_avg_feel) / (weather_history.update_count + 1),
        uvi_avg = (weather_history.uvi_avg * weather_history.update_count + EXCLUDED.uvi_avg) / (weather_history.update_count + 1),
        pressure_avg = (weather_history.pressure_avg * weather_history.update_count + EXCLUDED.pressure_avg) / (weather_history.update_count + 1),
        wind_speed_avg = (weather_history.wind_speed_avg * weather_history.update_count + EXCLUDED.wind_speed_avg) / (weather_history.update_count + 1),
        weather_description = EXCLUDED.weather_description,
        raw_data = EXCLUDED.raw_data,
		updated_at = CURRENT_TIMESTAMP,
        update_count = weather_history.update_count + 1;`

	var rain float64
	description := "Unknown"
	icon, main, summary := "", "", ""

	if len(wr.Report.Data.Daily) > 0 {
		rain = wr.Report.Data.Daily[0].Rain
		summary = wr.Report.Data.Daily[0].Summary
		if len(wr.Report.Data.Daily[0].Weather) > 0 {
			icon = wr.Report.Data.Daily[0].Weather[0].Icon
			main = wr.Report.Data.Daily[0].Weather[0].Main
			description = wr.Report.Data.Daily[0].Weather[0].Description
		}
	}

	if description == "Unknown" && len(wr.Report.Data.Current.Weather) > 0 {
		description = wr.Report.Data.Current.Weather[0].Description
	}

	historyData := map[string]any{
		"timezone":        wr.Report.Data.Timezone,
		"timezone_offset": wr.Report.Data.TimezoneOffset,
		"sunrise":         wr.Report.Data.Current.Sunrise,
		"sunset":          wr.Report.Data.Current.Sunset,
		"rain":            rain,
		"icon":            icon,
		"main":            main,
		"summary":         summary,
		"temp":            wr.Report.Data.Current.Temp,
	}

	_, err := wS.DB.Pool.Exec(ctx, query,
		wr.LocationId,                    // $1
		wr.Report.Data.Current.Temp,      // $2
		wr.Report.Data.Current.FeelsLike, // $3
		wr.Report.Data.Current.Uvi,       // $4
		wr.Report.Data.Current.Pressure,  // $5
		wr.Report.Data.Current.WindSpeed, // $6
		description,                      // $7
		historyData,                      // $8
	)
	return err
}

func (wS *WeatherService) FindWeatherCacheByLocId(ctx context.Context, locID int64, ttl int16) (*exApi.WeatherReport, []byte, error) {
	slog.Info("db find cache")
	const query = `
    SELECT full_data
    FROM weather_current_cache 
    WHERE location_id = $1 
      AND updated_at > NOW() - ($2 * interval '1 minute');`

	report := &exApi.WeatherReport{}
	var dataRaw []byte
	err := wS.DB.Pool.QueryRow(ctx, query, locID, ttl).Scan(&dataRaw)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	if err := sonic.Unmarshal(dataRaw, report); err != nil {
		return nil, nil, err
	}

	return report, dataRaw, err
}
