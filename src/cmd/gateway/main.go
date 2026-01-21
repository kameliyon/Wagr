package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"wagr/src/internal/fantasy/sleeper"
)

func main() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Sleeper routes
	sleeperHandlers := sleeper.NewHandlers(sleeper.NewClient())
	r.Route("/api/sleeper", sleeperHandlers.RegisterRoutes)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Println("Starting API Gateway on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
