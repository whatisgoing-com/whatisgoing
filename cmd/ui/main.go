package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
)

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<title>whatisgoing.com</title>
	<script src="https://unpkg.com/htmx.org@2.0.3"></script>
</head>
<body>
	<h1>whatisgoing.com</h1>
	<p>UI service is up.</p>
</body>
</html>`))

func main() {
	port := os.Getenv("UI_PORT")
	if port == "" {
		port = "8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /{$}", handleIndex)

	log.Printf("ui service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := indexTemplate.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
