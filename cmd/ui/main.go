package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/ui/coreclient"
)

const (
	defaultTrendingLimit       = 20
	defaultSearchLimit         = 20
	defaultOverallTrendDays    = 30
	defaultEntitySearchHits    = 10
	defaultRecentArticlesLimit = 10
)

var validWindows = map[string]bool{"day": true, "week": true, "month": true, "year": true}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := envOr("UI_PORT", "8081")
	core := coreclient.NewClient(mustEnv("CORE_API_URL"), nil)
	h := &handlers{core: core}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /{$}", h.handleTrending)
	mux.HandleFunc("GET /entities/search", h.handleEntitySearch)
	mux.HandleFunc("GET /entities/{id}", h.handleEntityDetail)
	mux.HandleFunc("GET /search", h.handleSearch)
	// Compiled by the standalone Tailwind CLI at Docker build time (see
	// cmd/ui/Dockerfile) and placed at this fixed path in the final image
	// — served from disk rather than go:embed so `go build`/`go test`
	// never depend on the CSS having been compiled first.
	mux.HandleFunc("GET /static/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/static/style.css")
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}

	go func() {
		log.Printf("ui service listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

type handlers struct {
	core *coreclient.Client
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *handlers) handleTrending(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	if !validWindows[window] {
		window = "day"
	}

	entities, err := h.core.Trending(r.Context(), window, defaultTrendingLimit)
	if err != nil {
		log.Printf("ui: trending: %v", err)
		http.Error(w, "failed to load trending entities", http.StatusBadGateway)
		return
	}

	overall, err := h.core.OverallTrend(r.Context(), window, defaultOverallTrendDays)
	if err != nil {
		log.Printf("ui: overall trend: %v", err)
		http.Error(w, "failed to load trend", http.StatusBadGateway)
		return
	}

	breakdown, err := h.core.SentimentBreakdown(r.Context(), window)
	if err != nil {
		log.Printf("ui: sentiment breakdown: %v", err)
		http.Error(w, "failed to load sentiment breakdown", http.StatusBadGateway)
		return
	}

	recentArticles, err := h.core.RecentArticles(r.Context(), 0, defaultRecentArticlesLimit)
	if err != nil {
		log.Printf("ui: recent articles: %v", err)
		http.Error(w, "failed to load recent articles", http.StatusBadGateway)
		return
	}

	data := buildTrendingData(window, entities, overall, breakdown, recentArticles)

	if r.Header.Get("HX-Request") == "true" {
		renderPartial(w, "trendingPanel", data)
		return
	}
	renderPage(w, "trendingContent", data)
}

func (h *handlers) handleEntitySearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var results []coreclient.EntitySummary
	if query != "" {
		var err error
		results, err = h.core.SearchEntities(r.Context(), query, defaultEntitySearchHits)
		if err != nil {
			log.Printf("ui: entity search: %v", err)
			http.Error(w, "entity search failed", http.StatusBadGateway)
			return
		}
	}

	renderPartial(w, "entitySearchResults", results)
}

func (h *handlers) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid entity id", http.StatusBadRequest)
		return
	}

	window := r.URL.Query().Get("window")
	if !validWindows[window] {
		window = "day"
	}

	detail, found, err := h.core.EntityDetail(r.Context(), id, window)
	if err != nil {
		log.Printf("ui: entity detail: %v", err)
		http.Error(w, "failed to load entity", http.StatusBadGateway)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		renderPage(w, "entityNotFound", nil)
		return
	}

	sourceBreakdown, err := h.core.SourceBreakdown(r.Context(), id)
	if err != nil {
		log.Printf("ui: source breakdown: %v", err)
		http.Error(w, "failed to load source breakdown", http.StatusBadGateway)
		return
	}

	recentArticles, err := h.core.RecentArticles(r.Context(), id, defaultRecentArticlesLimit)
	if err != nil {
		log.Printf("ui: recent articles: %v", err)
		http.Error(w, "failed to load recent articles", http.StatusBadGateway)
		return
	}

	renderPage(w, "entityContent", buildEntityPageData(detail, sourceBreakdown, recentArticles))
}

func (h *handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	data := searchData{Query: query}
	if query != "" {
		results, err := h.core.Search(r.Context(), query, defaultSearchLimit)
		if err != nil {
			log.Printf("ui: search: %v", err)
			http.Error(w, "search failed", http.StatusBadGateway)
			return
		}
		data.Searched = true
		data.Results = results
	}

	if r.Header.Get("HX-Request") == "true" {
		renderPartial(w, "searchResults", data)
		return
	}
	renderPage(w, "searchContent", data)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
