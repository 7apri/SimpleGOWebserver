package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
)

// //go:embed public/templates/* public/static/*
// var webAssets embed.FS
var templates *template.Template

/*
func init() {
	templates = template.Must(template.ParseFS(webAssets, "public/templates/index.html", "public/templates/404.html"))
}
*/

type Server struct {
	locationService *LocationService
	weatherService  *WeatherService
	database        *Database
}

func main() {
	weatherApiKey = os.Getenv("WEATHER_API_KEY")
	if weatherApiKey == "" {
		fmt.Fprint(os.Stderr, "Weather API key is empty please check the .env")
		os.Exit(1)
	}

	db := InitDB()
	defer db.Close()
	ls, err := NewLocationService(db, 500)
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error creating the location service: %s", err)
		os.Exit(1)
	}
	ws, err := NewWeatherService(db, 500)
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error creating the weather service: %s", err)
		os.Exit(1)
	}

	srv := &Server{
		locationService: ls,
		weatherService:  ws,
		database:        db,
	}

	http.HandleFunc("/", HandleRoot)

	http.HandleFunc("/api/health", srv.HandleHealth)

	http.HandleFunc("/api/weather", srv.HandleWeather)
	http.HandleFunc("/api/location", srv.HandleLocation)

	http.HandleFunc("/api/login", srv.HandleLogin)
	http.HandleFunc("/api/register", srv.HandleRegister)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Printf("Server starting on %s:80 (external:inernal)\n", os.Getenv("SERVER_PORT"))

	err = http.ListenAndServe(":80", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error running the server: %s", err)
		os.Exit(1)
	}
}
