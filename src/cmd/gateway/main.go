package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	// HederaClient is optional: if the operator key is not configured, payout execution
	// methods will return ErrMissingOperatorKey rather than panicking.
	var hederaClient *league.HederaClient
	if operatorID := os.Getenv("HEDERA_OPERATOR_ID"); operatorID != "" {
		if operatorKey := os.Getenv("HEDERA_OPERATOR_KEY"); operatorKey != "" {
			hc, err := league.NewHederaClient(hederaNetwork, operatorID, operatorKey, hederaEscrowContractID)
			if err != nil {
				log.Printf("Warning: failed to initialize Hedera operator client: %v", err)
			} else {
				hederaClient = hc
				log.Println("Hedera operator client initialized")
			}
		}
	}

	leagueService := league.NewService(db.Pool, platformService, hederaUSDCTokenID, hederaEscrowContractID, hederaNetwork, hederaClient)
	leagueHandlers := league.NewHandler(leagueService)
	r.Route("/api/leagues", func(r chi.Router) {
		leagueHandlers.RegisterRoutes(r, *authHandlers)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Start HTTP server in background so we can handle shutdown signals
	server := &http.Server{Addr: ":8080", Handler: r}
	go func() {
		log.Println("Starting API Gateway on :8080")
		log.Println("Routes registered:")
		log.Println("  - /api/auth/* (authentication)")
		log.Println("  - /api/fantasy/* (platform-agnostic)")
		log.Println("  - /api/leagues/* (league management - authenticated)")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Block until SIGINT or SIGTERM
	<-ctx.Done()
	stop()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}

