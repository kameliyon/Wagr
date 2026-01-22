package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"wagr/src/internal/auth"
	"wagr/src/internal/database"
	"wagr/src/internal/fantasy/sleeper"
)

func main() {
	// Initialize database
	dbConfig := database.DefaultConfig()
	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS configuration for frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Auth routes
	authService := auth.NewService(db.Pool)
	authHandlers := auth.NewHandlers(authService)
	r.Route("/api/auth", authHandlers.RegisterRoutes)

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
