package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/trace"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/api"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/server"
	"github.com/7apri/SimpleGOWebserver/internal/services"
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

	owClient := api.NewOwClient(weatherApiKey, (24*time.Hour)/1000)

	ls, err := services.NewLocationService(db, 500, weatherApiKey, owClient, api.NewIpClient(time.Minute/40))
	if err != nil {
		slog.Error("There was an error creating the location service", "error", err)
		os.Exit(1)
	}
	ws, err := services.NewWeatherService(db, 500)
	if err != nil {
		slog.Error("There was an error creating the weather service", "error", err)
		os.Exit(1)
	}

	srv := &server.Server{
		LocationService: ls,
		WeatherService:  ws,
		Database:        db,
	}

	http.HandleFunc("/", srv.HandleRoot)

	http.HandleFunc("/api/health", srv.HandleHealth)

	http.HandleFunc("/api/weather", srv.HandleWeather)
	http.HandleFunc("/api/location", srv.HandleLocation)

	http.HandleFunc("/api/login", srv.HandleLogin)
	http.HandleFunc("/api/register", srv.HandleRegister)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	slog.Info(fmt.Sprintf("Server starting on %s:80 (external:inernal)", os.Getenv("SERVER_PORT")))

	err = http.ListenAndServe(":80", nil)
	if err != nil {
		slog.Error("There was an error running the server", "error", err)
		os.Exit(1)
	}
}
