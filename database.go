package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var ddlSchema string

type Database struct {
	*sql.DB
}

func InitDB() *Database {
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	baseUrl := os.Getenv("DB_BASE_URL")

	dbSocketPath := fmt.Sprintf("%s user=%s password=%s dbname=%s sslmode=disable",
		baseUrl, user, pass, name)

	rawDB, err := sql.Open("pgx", dbSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	err = rawDB.Ping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not reach database: %v\n", err)
		os.Exit(1)
	}

	rawDB.SetMaxOpenConns(10)
	rawDB.SetMaxIdleConns(10)
	rawDB.SetConnMaxLifetime(5 * time.Minute)

	_, err = rawDB.Exec(ddlSchema)
	if err != nil {
		panic(fmt.Sprintf("Failed to run schema migration: %v", err))
	}

	fmt.Println("Successfully connected to Postgres!")
	return &Database{rawDB}
}

func (db *Database) GetLatency() (string, error) {
	start := time.Now()

	if err := db.Ping(); err != nil {
		return "", err
	}

	return time.Since(start).String(), nil
}

func (db *Database) SaveLocation(location *GeoResult) error {
	query := `
        INSERT INTO locations (city_name, state, country, lat, lon,local_names)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT DO NOTHING`

	var state any = location.State
	if location.State == "" {
		state = nil
	}

	namesJson, err := json.Marshal(location.LocalNames)
	if err != nil {
		return fmt.Errorf("failed to marshal local names: %w", err)
	}

	_, err = db.Exec(query, location.CityName, state, location.Country, location.Lat, location.Lon, namesJson)
	if err != nil {
		return fmt.Errorf("failed to save location: %w", err)
	}

	return nil
}

func (db *Database) FindLocationByCoords(coords *Coordinates) (*LocationReadableAdress, error) {
	if coords == nil {
		return nil, errors.New("coordinates cannot be nil")
	}

	const threshold float64 = 0.005
	query := `
    SELECT city_name, state, country 
    FROM locations 
    WHERE lat BETWEEN ($1::float - $3::float) AND ($1::float + $3::float)
      AND lon BETWEEN ($2::float - $3::float) AND ($2::float + $3::float)
    LIMIT 1`

	var loc LocationReadableAdress
	var state sql.NullString

	err := db.QueryRow(query, coords.Lat, coords.Lon, threshold).Scan(&loc.CityName, &state, &loc.Country)
	if err != nil {
		return nil, err
	}

	loc.State = state.String
	return &loc, nil
}

/*
func (db *Database) FindWeatherByLocation() (*LocationReadableAdress, error) {
	if coords == nil {
		return nil, errors.New("coordinates cannot be nil")
	}

	const threshold float64 = 0.005
	query := `
    SELECT city_name, state, country
    FROM locations
    WHERE lat BETWEEN ($1::float - $3::float) AND ($1::float + $3::float)
      AND lon BETWEEN ($2::float - $3::float) AND ($2::float + $3::float)
    LIMIT 1`

	var loc LocationReadableAdress
	var state sql.NullString

	err := db.QueryRow(query, coords.Lat, coords.Lon, threshold).Scan(&loc.CityName, &state, &loc.Country)
	if err != nil {
		return nil, err
	}

	loc.State = state.String
	return &loc, nil
}
*/
