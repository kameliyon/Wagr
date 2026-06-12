package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"

	"wagr/src/internal/database"
	"wagr/src/internal/fantasy"
	"wagr/src/internal/fantasy/sleeper"
	"wagr/src/internal/league"
)

// defaultJobTimeout caps the total runtime of a single oracle invocation.
// Set via ORACLE_JOB_TIMEOUT env var (e.g. "15m"). Defaults to 10 minutes.
const defaultJobTimeout = 10 * time.Minute

func main() {
	job := flag.String("job", "", `which job to run: "weekly", "season", or omit to run both`)
	flag.Parse()

	switch *job {
	case "", "weekly", "season":
		// valid
	default:
		log.Fatalf("[oracle] unknown -job value %q: must be \"weekly\", \"season\", or omitted", *job)
	}

	if envPath := findEnvFile(); envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Warning: could not load %s: %v", envPath, err)
		}
	}

	timeout := defaultJobTimeout
	if v := os.Getenv("ORACLE_JOB_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dbConfig := database.DefaultConfig()
	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("[oracle] failed to connect to database: %v", err)
	}
	defer db.Close()

	registry := fantasy.NewRegistry()
	registry.Register(sleeper.NewAdapter(sleeper.NewClient()))
	platformService := fantasy.NewPlatformService(registry)

	hederaNetwork := os.Getenv("HEDERA_NETWORK")
	hederaEscrowContractID := os.Getenv("HEDERA_ESCROW_CONTRACT_ID")
	hederaUSDCTokenID := os.Getenv("HEDERA_USDC_TOKEN_ID")

	var hederaClient *league.HederaClient
	if operatorID := os.Getenv("HEDERA_OPERATOR_ID"); operatorID != "" {
		if operatorKey := os.Getenv("HEDERA_OPERATOR_KEY"); operatorKey != "" {
			hc, err := league.NewHederaClient(hederaNetwork, operatorID, operatorKey, hederaEscrowContractID)
			if err != nil {
				log.Fatalf("[oracle] failed to initialize Hedera client: %v", err)
			}
			hederaClient = hc
		}
	}
	if hederaClient == nil {
		log.Fatal("[oracle] HEDERA_OPERATOR_ID and HEDERA_OPERATOR_KEY must be set")
	}

	svc := league.NewService(db.Pool, platformService, hederaUSDCTokenID, hederaEscrowContractID, hederaNetwork, hederaClient)

	failed := false

	if *job == "" || *job == "weekly" {
		log.Println("[oracle] running weekly payout job")
		if err := svc.RunWeeklyPayoutJob(ctx); err != nil {
			log.Printf("[oracle] weekly payout job error: %v", err)
			failed = true
		}
	}

	if *job == "" || *job == "season" {
		log.Println("[oracle] running season-end payout job")
		if err := svc.RunSeasonEndPayoutJob(ctx); err != nil {
			log.Printf("[oracle] season-end payout job error: %v", err)
			failed = true
		}
	}

	if failed {
		log.Println("[oracle] completed with errors")
		os.Exit(1)
	}
	log.Println("[oracle] completed successfully")
}

// findEnvFile walks up from the executable's directory to find a .env file.
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
