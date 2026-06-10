package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"wagr/src/internal/auth"
	"wagr/src/internal/database"
	"wagr/src/internal/fantasy"
	"wagr/src/internal/fantasy/sleeper"
	"wagr/src/internal/league"
)

func main() {
	// if envPath := findEnvFile(); envPath != "" {
	// 	if err := godotenv.Overload(envPath); err == nil {
	// 		log.Printf("Loaded environment from %s", envPath)
	// 	}
	// } else {
	// 	log.Println("No .env file found, using environment variables")
	// }

	err := godotenv.Load("../.env")
	if err != nil {
		log.Printf("No .env file found, using environment variables")
	}

	// Initialize database
	dbConfig := database.DefaultConfig()
	log.Printf("Connecting to database at %s:%s", dbConfig.Host, dbConfig.Port)
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
	r.Route("/api/fantasy", fantasyHandlers.RegisterRoutes) 

	// League management routes (NEW - requires authentication)
	hederaUSDCTokenID := os.Getenv("HEDERA_USDC_TOKEN_ID")
	hederaEscrowContractID := os.Getenv("HEDERA_ESCROW_CONTRACT_ID")
	hederaNetwork := os.Getenv("HEDERA_NETWORK")
	leagueService := league.NewService(db.Pool, platformService, hederaUSDCTokenID, hederaEscrowContractID, hederaNetwork)
	leagueHandlers := league.NewHandler(leagueService)
	r.Route("/api/leagues", func(r chi.Router){
		leagueHandlers.RegisterRoutes(r, *authHandlers)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Println("Starting API Gateway on :8080")
	log.Println("Routes registered:")
	log.Println("  - /api/auth/* (authentication)")
	log.Println("  - /api/fantasy/* (platform-agnostic)")
	log.Println("  - /api/leagues/* (league management - authenticated)")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// findEnvFile walks up from the current directory to find a .env file
// next to go.mod, so the app loads config regardless of which directory it's run from.
func findEnvFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
