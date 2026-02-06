package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/trace"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/server"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
)

// //go:embed public/templates/* public/static/*
// var webAssets embed.FS
// var templates *template.Template

/*
func init() {
	templates = template.Must(template.ParseFS(webAssets, "public/templates/index.html", "public/templates/404.html"))
}
*/

func main() {
	f, _ := os.Create("trace.out")
	trace.Start(f)
	defer trace.Stop()

	weatherApiKey := os.Getenv("WEATHER_API_KEY")
	if weatherApiKey == "" {
		slog.Error("weather API key is empty please check the .env")
		os.Exit(1)
	}

	db := database.InitDB()
	defer db.Pool.Close()

	owClient := exApi.NewOwClient(weatherApiKey, time.Microsecond) //(24*time.Hour)/1000

	ls, err := location.NewLocationService(db, 500, owClient, exApi.NewIpClient(time.Minute/40))
	if err != nil {
		slog.Error("There was an error creating the location service", "error", err)
		os.Exit(1)
	}
	ws, err := weather.NewWeatherService(db, 500, owClient, ls)
	if err != nil {
		slog.Error("There was an error creating the weather service", "error", err)
		os.Exit(1)
	}

	ah := auth.NewAuthHandler(db.Pool)

	srv := &server.Server{
		LocationService: ls,
		WeatherService:  ws,
		Database:        db,
		AuthHandler:     ah,
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	slog.Info(fmt.Sprintf("Server starting on %s:80 (external:inernal)", os.Getenv("SERVER_PORT")))

	err = http.ListenAndServe(":80", srv.Routes())
	if err != nil {
		slog.Error("There was an error running the server", "error", err)
		os.Exit(1)
	}
}
