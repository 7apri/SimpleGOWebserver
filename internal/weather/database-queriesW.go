package weather

import (
	"context"
	"log/slog"
	"time"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

const cacheQ = `
		INSERT INTO weather_current_cache (location_id, full_data, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (location_id) 
		DO UPDATE SET 
			full_data = EXCLUDED.full_data,
			updated_at = NOW();`
const historyQ = `
    INSERT INTO weather_history (
		location_id,recorded_date, 	-- $1,$2
		temp_avg,temp_avg_feel, 	-- $3,$4
		uvi_avg,pressure_avg, 		-- $5,$6
		wind_speed_avg,humidity_avg,-- $7,$8
		temp_morning,temp_day, 		-- $9,$10
		temp_evening,temp_night,	-- $11,$12
		temp_min,temp_max,			-- $13,$14
		weather_id, 				-- $15
		raw_data,is_forecast		-- $16,$17
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	ON CONFLICT (location_id, recorded_date) 
	DO UPDATE SET
	
	temp_avg = CASE WHEN weather_history.is_forecast = TRUE AND EXCLUDED.is_forecast = FALSE THEN EXCLUDED.temp_avg
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.temp_avg * weather_history.update_count + EXCLUDED.temp_avg) / (weather_history.update_count + 1)
		ELSE weather_history.temp_avg END,
	
	temp_avg_feel = CASE WHEN weather_history.is_forecast = TRUE AND EXCLUDED.is_forecast = FALSE THEN EXCLUDED.temp_avg_feel
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.temp_avg_feel * weather_history.update_count + EXCLUDED.temp_avg_feel) / (weather_history.update_count + 1)
		ELSE weather_history.temp_avg_feel END,
	
	uvi_avg = CASE WHEN weather_history.is_forecast = TRUE THEN EXCLUDED.uvi_avg
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.uvi_avg * weather_history.update_count + EXCLUDED.uvi_avg) / (weather_history.update_count + 1)
		ELSE weather_history.uvi_avg END,
																		
	pressure_avg = CASE WHEN weather_history.is_forecast = TRUE THEN EXCLUDED.pressure_avg
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.pressure_avg * weather_history.update_count + EXCLUDED.pressure_avg) / (weather_history.update_count + 1)
		ELSE weather_history.pressure_avg END,
	
	wind_speed_avg  = CASE WHEN weather_history.is_forecast = TRUE THEN EXCLUDED.wind_speed_avg
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.wind_speed_avg  * weather_history.update_count + EXCLUDED.wind_speed_avg ) / (weather_history.update_count + 1)
		ELSE weather_history.wind_speed_avg END,
																		
	humidity_avg = CASE WHEN weather_history.is_forecast = TRUE THEN EXCLUDED.humidity_avg
		WHEN EXCLUDED.is_forecast = FALSE THEN (weather_history.humidity_avg * weather_history.update_count + EXCLUDED.humidity_avg) / (weather_history.update_count + 1)
		ELSE weather_history.humidity_avg END,	

	temp_morning = 	EXCLUDED.temp_morning,
    temp_day = 		EXCLUDED.temp_day,
    temp_evening = 	EXCLUDED.temp_evening,
    temp_night = 	EXCLUDED.temp_night,
    temp_min = 		EXCLUDED.temp_min,
    temp_max = 		EXCLUDED.temp_max,

    weather_id = 	EXCLUDED.weather_id,
    raw_data = 		EXCLUDED.raw_data,
    updated_at = 	CURRENT_TIMESTAMP,
    is_forecast = 	(weather_history.is_forecast AND EXCLUDED.is_forecast),
	update_count = 	CASE WHEN weather_history.is_forecast = TRUE AND EXCLUDED.is_forecast = FALSE THEN 1 
						 WHEN EXCLUDED.is_forecast = FALSE THEN weather_history.update_count + 1 
						 ELSE weather_history.update_count END;`

const findQId = `
    SELECT full_data
    FROM weather_current_cache 
    WHERE location_id = $1 
      AND updated_at > NOW() - ($2 * interval '1 minute');`

func (wS *WeatherService) FindWeatherCacheByLocId(ctx context.Context, locID int64, ttl int16) (*exApi.WeatherReport, []byte, error) {

	report := &exApi.WeatherReport{}
	var dataRaw []byte
	err := wS.DB.Pool.QueryRow(ctx, findQId, locID, ttl).Scan(&dataRaw)

	if err != nil {
		return nil, nil, err
	}

	if err := sonic.Unmarshal(dataRaw, report); err != nil {
		return nil, nil, err
	}

	return report, dataRaw, err
}

const findQAddress = `
SELECT l.id, w.full_data
FROM weather_current_cache w
JOIN locations l ON l.id = w.location_id
WHERE l.city_name = $1 
  AND l.country = $2 
  AND COALESCE(l.state, '') = $3
  AND w.updated_at > NOW() - ($4 * interval '1 minute');`

func (wS *WeatherService) FindWeatherCacheByAddress(ctx context.Context, address *exApi.LocationReadableAddress, ttl int16) (int64, *exApi.WeatherReport, []byte, error) {
	report := &exApi.WeatherReport{}
	var (
		locID   int64
		dataRaw []byte
	)

	err := wS.DB.Pool.QueryRow(
		ctx,
		findQAddress,
		address.CityName,
		address.Country,
		address.State,
		ttl,
	).Scan(&locID, &dataRaw)

	if err != nil {
		return 0, nil, nil, err
	}

	if err := sonic.Unmarshal(dataRaw, report); err != nil {
		return locID, nil, nil, err
	}

	return locID, report, dataRaw, nil
}

func addIfPresent(m map[string]any, key string, val any) {
	if val != nil {
		m[key] = val
	}
}

func (wS *WeatherService) flush(ctx context.Context, batch []exApi.WeatherReportGeoRes, b *pgx.Batch) {
	for _, item := range batch {
		data := item.Report.Data
		if data.Current == nil {
			continue
		}

		locID := item.GeoRes.GetId()
		b.Queue(cacheQ,
			locID, item.Report,
		)

		timezone := time.FixedZone("Local", data.TimezoneOffset)
		currentLocal := time.Unix(data.Current.Dt, 0).In(timezone)

		var tMorn, tDay, tEve, tNight, tMin, tMax *float64
		var currentDayId *int16

		rawCurrent := make(map[string]any)
		addIfPresent(rawCurrent, "sunrise", data.Current.Sunrise)
		addIfPresent(rawCurrent, "sunset", data.Current.Sunset)

		if len(data.Daily) > 0 {
			for _, day := range data.Daily {
				dayLocal := time.Unix(day.Dt, 0).In(timezone)
				rawDay := make(map[string]any)
				var dayId int16
				if len(day.Weather) > 0 {
					dayId = day.Weather[0].ID
				}

				if dayLocal.YearDay() == currentLocal.YearDay() && dayLocal.Year() == currentLocal.Year() {
					tMorn, tDay, tEve, tNight = &day.Temp.Morning, &day.Temp.Day, &day.Temp.Evening, &day.Temp.Night
					tMin, tMax = &day.Temp.Min, &day.Temp.Max
					if len(day.Weather) > 0 {
						currentDayId = &day.Weather[0].ID
					}
					addIfPresent(rawCurrent, "moonrise", day.Moonrise)
					addIfPresent(rawCurrent, "moonset", day.Moonset)
					addIfPresent(rawCurrent, "moon_phase", day.MoonPhase)
					addIfPresent(rawCurrent, "dew_point", day.DewPoint)
					addIfPresent(rawCurrent, "snow", day.Snow)
					addIfPresent(rawCurrent, "rain", day.Rain)
					addIfPresent(rawCurrent, "clouds", day.Clouds)

					continue
				}

				addIfPresent(rawDay, "sunrise", data.Current.Sunrise)
				addIfPresent(rawDay, "sunset", data.Current.Sunset)
				addIfPresent(rawDay, "moonrise", day.Moonrise)
				addIfPresent(rawDay, "moonset", day.Moonset)
				addIfPresent(rawDay, "moon_phase", day.MoonPhase)
				addIfPresent(rawDay, "dew_point", day.DewPoint)
				addIfPresent(rawDay, "snow", day.Snow)
				addIfPresent(rawDay, "rain", day.Rain)
				addIfPresent(rawDay, "clouds", day.Clouds)

				b.Queue(historyQ,
					locID,
					dayLocal,
					nil,
					nil,
					day.Uvi,
					day.Pressure,
					day.WindSpeed,
					day.Humidity,
					day.Temp.Morning,
					day.Temp.Day,
					day.Temp.Evening,
					day.Temp.Night,
					day.Temp.Min,
					day.Temp.Max,
					dayId,
					rawDay,
					true,
				)
			}
		}

		b.Queue(historyQ,
			locID,
			currentLocal,
			data.Current.Temp,
			data.Current.FeelsLike,
			data.Current.Uvi,
			data.Current.Pressure,
			data.Current.WindSpeed,
			data.Current.Humidity,
			tMorn,
			tDay,
			tEve,
			tNight,
			tMin,
			tMax,
			currentDayId,
			rawCurrent,
			false,
		)
		wS.wg.Done()
	}

	res := wS.DB.Pool.SendBatch(ctx, b)
	defer res.Close()

	for i := range b.Len() {
		_, err := res.Exec()
		if err != nil {
			slog.Error("Batch execution failed", "error", err, "index", i)
		}
	}
}
