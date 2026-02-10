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
	"wagr/src/internal/fantasy"
	"wagr/src/internal/fantasy/sleeper"
	"wagr/src/internal/league"
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

	// Initialize platform registry
	registry := fantasy.NewRegistry()

	// Register Sleeper adapter
	sleeperClient := sleeper.NewClient()
	sleeperAdapter := sleeper.NewAdapter(sleeperClient)
	registry.Register(sleeperAdapter)
	log.Println("Registered Sleeper platform adapter")

	// Create platform service
	platformService := fantasy.NewPlatformService(registry)

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

	// Auth service and middleware
	authService := auth.NewService(db.Pool)
	authHandlers := auth.NewHandlers(authService)

	// Auth routes
	r.Route("/api/auth", authHandlers.RegisterRoutes)

	// Platform-agnostic fantasy routes (NEW)
	fantasyHandlers := fantasy.NewHandler(platformService)
	r.Route("/api/fantasy", func(r chi.Router) {
		r.Get("/platforms", fantasyHandlers.ListPlatforms)
		r.Get("/{platform}/user/{identifier}", fantasyHandlers.GetUser)
		r.Get("/{platform}/user/{userId}/leagues", fantasyHandlers.GetUserLeagues)
		r.Get("/{platform}/league/{leagueId}", fantasyHandlers.GetLeague)
		r.Get("/{platform}/league/{leagueId}/members", fantasyHandlers.GetLeagueMembers)
		r.Get("/{platform}/league/{leagueId}/rosters", fantasyHandlers.GetLeagueRosters)
	})

	// League management routes (NEW - requires authentication)
	leagueService := league.NewService(db.Pool, platformService)
	leagueHandlers := league.NewHandler(leagueService)
	r.Route("/api/leagues", func(r chi.Router) {
		r.Use(authHandlers.AuthMiddleware)
		r.Post("/link-platform", leagueHandlers.LinkPlatform)
		r.Post("/import", leagueHandlers.ImportLeague)
		r.Get("/", leagueHandlers.GetUserLeagues)
		r.Get("/{leagueId}", leagueHandlers.GetLeague)
	})

	// Legacy Sleeper routes (backward compatibility)
	sleeperHandlers := sleeper.NewHandlers(sleeperClient)
	r.Route("/api/sleeper", sleeperHandlers.RegisterRoutes)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Println("Starting API Gateway on :8080")
	log.Println("Routes registered:")
	log.Println("  - /api/auth/* (authentication)")
	log.Println("  - /api/fantasy/* (platform-agnostic)")
	log.Println("  - /api/leagues/* (league management - authenticated)")
	log.Println("  - /api/sleeper/* (legacy - backward compatibility)")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
