package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
)

//go:embed public/templates/* public/static/*
var webAssets embed.FS
var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(webAssets, "public/templates/index.html", "public/templates/404.html"))
}

func main() {
	InitDB()
	defer DB.Close()

	http.HandleFunc("/", HandleRoot)
	http.HandleFunc("/health", HandleHealth)

	fs := http.FileServer(http.FS(webAssets))
	http.Handle("/static/", fs)

	fmt.Println("Server starting on :80...")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "There was an error running the server: %s", err)
		os.Exit(1)
	}
}
