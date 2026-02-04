package database

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	util "github.com/7apri/SimpleGOWebserver/pkg"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var ddlSchema string

type Database struct {
	Pool *pgxpool.Pool
}

func InitDB() *Database {
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect: %v\n", err)
		os.Exit(1)
	}

	_, err = pool.Exec(context.TODO(), ddlSchema)
	if err != nil {
		panic(fmt.Sprintf("Failed to run schema migration: %v", err))
	}

	fmt.Println("Successfully connected to Postgres!")
	return &Database{pool}
}

func (db *Database) GetLatency() (string, error) {
	start := time.Now()

	if err := db.Pool.Ping(context.TODO()); err != nil {
		return "", err
	}

	return time.Since(start).String(), nil
}

func (db *Database) SaveLocation(loc *location.GeoResult) (locationId int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	query := `
        INSERT INTO locations (city_name, state, country, lat, lon, local_names)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (city_name, state, country) 
        DO UPDATE SET city_name = EXCLUDED.city_name
        RETURNING id`

	var namesJson []byte
	if len(loc.LocalNames) > 0 {
		namesJson, _ = sonic.Marshal(loc.LocalNames)
	}
	err = db.Pool.QueryRow(ctx, query,
		util.CleanQuery(loc.CityName),
		util.CleanQuery(loc.State),
		loc.Country,
		loc.Lat,
		loc.Lon,
		namesJson,
	).Scan(&locationId)
	if err != nil {
		return -1, err
	}
	return locationId, nil
}

func (db *Database) FindLocationByCoords(ctx context.Context, coords *location.Coordinates) (*location.GeoResult, error) {
	if coords == nil {
		return nil, errors.New("coordinates cannot be nil")
	}

	const threshold float64 = 0.005
	query := `
        SELECT id, city_name, state, country, lat, lon, local_names
        FROM locations
        WHERE lat BETWEEN ($1::float - $3::float) AND ($1::float + $3::float)
          AND lon BETWEEN ($2::float - $3::float) AND ($2::float + $3::float)
        LIMIT 1`

	var loc location.GeoResult
	var namesRaw []byte
	var locId int64

	err := db.Pool.QueryRow(ctx, query, coords.Lat, coords.Lon, threshold).Scan(
		&locId,
		&loc.CityName,
		&loc.State,
		&loc.Country,
		&loc.Lat,
		&loc.Lon,
		&namesRaw,
	)
	if err != nil {
		return nil, err
	}
	loc.Id.Store(locId)

	if len(namesRaw) > 0 {
		sonic.Unmarshal(namesRaw, &loc.LocalNames)
	}

	return &loc, nil
}

func (db *Database) FindLocationByAddress(ctx context.Context, locIN *location.LocationReadableAddress) (*location.GeoResult, error) {
	if locIN == nil {
		return nil, errors.New("location cannot be nil")
	}

	args := make([]any, 0, 3)
	args = append(args, locIN.CityName)
	args = append(args, locIN.Country)

	var b strings.Builder
	b.Grow(195)
	b.WriteString(`
	SELECT  id, city_name, state, country, lat, lon, local_names
    FROM locations
    WHERE to_tsvector('simple', city_name) @@ to_tsquery('simple', $1 || ':*')
      AND country = $2 
	`)
	if locIN.State != "" {
		b.WriteString("AND state = $3 ")
		args = append(args, locIN.State)
	}
	b.WriteString("LIMIT 1")

	var loc location.GeoResult
	var namesRaw []byte
	var locId int64

	err := db.Pool.QueryRow(ctx, b.String(), args...).Scan(
		&locId,
		&loc.CityName,
		&loc.State,
		&loc.Country,
		&loc.Lat,
		&loc.Lon,
		&namesRaw,
	)

	if err != nil {
		return nil, err
	}
	loc.Id.Store(locId)

	if len(namesRaw) > 0 {
		sonic.Unmarshal(namesRaw, &loc.LocalNames)
	}

	return &loc, nil
}

func (db *Database) SaveWeatherCache(weather *weather.WeatherReportId) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	query := `
        INSERT INTO weather_current_cache (location_id, full_data, updated_at)
        VALUES ($1, $2, NOW())
		ON CONFLICT (location_id) 
		DO UPDATE SET 
			full_data = EXCLUDED.full_data,
			updated_at = NOW();`

	_, err := db.Pool.Exec(ctx, query, weather.LocationId, weather.Report)
	return err
}
func (db *Database) SaveWeatherHistory(ctx context.Context, wr *weather.WeatherReportId) error {
	query := `
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

	_, err := db.Pool.Exec(ctx, query,
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

func (db *Database) FindWeatherCacheByLocId(ctx context.Context, locID int64, ttl int16) (*weather.WeatherReport, []byte, error) {
	query := `
    SELECT full_data
    FROM weather_current_cache 
    WHERE location_id = $1 
      AND updated_at > NOW() - ($2 * interval '1 minute');`

	report := &weather.WeatherReport{}
	var dataRaw []byte
	err := db.Pool.QueryRow(ctx, query, locID, ttl).Scan(&dataRaw)

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
