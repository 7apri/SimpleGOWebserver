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
	util "github.com/7apri/SimpleGOWebserver/pkg"
	"github.com/bytedance/sonic"
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

func (db *Database) SaveLocation(loc *location.GeoResult) error {
	query := `
        INSERT INTO locations (city_name, state, country, lat, lon, local_names)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (city_name, state, country) DO NOTHING`

	var namesJson []byte
	if len(loc.LocalNames) > 0 {
		namesJson, _ = sonic.Marshal(loc.LocalNames)
	}

	_, err := db.Pool.Exec(context.TODO(), query, util.CleanQuery(loc.CityName), util.CleanQuery(loc.State), loc.Country, loc.Lat, loc.Lon, namesJson)
	return err
}

func (db *Database) FindLocationByCoords(ctx context.Context, coords *location.Coordinates) (*location.GeoResult, error) {
	if coords == nil {
		return nil, errors.New("coordinates cannot be nil")
	}

	const threshold float64 = 0.005
	query := `
        SELECT city_name, state, country, lat, lon, local_names
        FROM locations
        WHERE lat BETWEEN ($1::float - $3::float) AND ($1::float + $3::float)
          AND lon BETWEEN ($2::float - $3::float) AND ($2::float + $3::float)
        LIMIT 1`

	var loc location.GeoResult
	var namesRaw []byte

	err := db.Pool.QueryRow(ctx, query, coords.Lat, coords.Lon, threshold).Scan(
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
	SELECT city_name, state, country, lat, lon, local_names
    FROM locations
    WHERE to_tsvector('simple', city_name) @@ to_tsquery('simple', $1 || ':*')
      AND country = $2 
	`)
	if locIN.State != "" {
		b.WriteString("AND state = $3")
		args = append(args, locIN.State)
	}
	b.WriteString("LIMIT 1")

	var loc location.GeoResult
	var namesRaw []byte

	err := db.Pool.QueryRow(ctx, b.String(), args...).Scan(
		&loc.CityName, &loc.State, &loc.Country, &loc.Lat, &loc.Lon, &namesRaw,
	)

	if err != nil {
		return nil, err
	}

	if len(namesRaw) > 0 {
		sonic.Unmarshal(namesRaw, &loc.LocalNames)
	}

	return &loc, nil
}
