package main

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var ddlSchema string

var DB *sql.DB

func InitDB() {
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	baseUrl := os.Getenv("DB_BASE_URL")

	dbSocketPath := fmt.Sprintf("%s user=%s password=%s dbname=%s sslmode=disable",
		baseUrl, user, pass, name)

	var err error
	DB, err = sql.Open("pgx", dbSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	err = DB.Ping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not reach database: %v\n", err)
		os.Exit(1)
	}

	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * time.Minute)

	_, err = DB.Exec(ddlSchema)
	if err != nil {
		panic(fmt.Sprintf("Failed to run schema migration: %v", err))
	}

	fmt.Println("Successfully connected to Postgres!")
}

func GetLatency() (string, error) {
	start := time.Now()

	if err := DB.Ping(); err != nil {
		return "", err
	}

	return time.Since(start).String(), nil
}
