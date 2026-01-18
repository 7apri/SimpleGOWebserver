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

func main() {
	weatherApiKey = os.Getenv("WEATHER_API_KEY")
	if weatherApiKey == "" {
		fmt.Fprint(os.Stderr, "Weather API key is empty please check the .env")
		os.Exit(1)
	}

	InitDB()
	defer DB.Close()

	http.HandleFunc("/", HandleRoot)

	http.HandleFunc("/api/health", HandleHealth)
	http.HandleFunc("/api/weather", HandleWeather)

	http.HandleFunc("/api/login", HandleHealth)
	http.HandleFunc("/api/register", HandleWeather)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Printf("Server starting on %s:80 (external:inernal)\n", os.Getenv("SERVER_PORT"))

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error running the server: %s", err)
		os.Exit(1)
	}
}
