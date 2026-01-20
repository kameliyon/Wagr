# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WAGR is a Web3 application for managing payments for Fantasy sports leagues. Users can create leagues, connect wallets, and handle entry fees and payouts via smart contracts on blockchain (Ethereum Testnet/Cardano).

## Technology Stack

- **Frontend**: React/Next.js (planned)
- **Backend**: Go microservices with Chi router
- **Database**: PostgreSQL with Redis caching (planned)
- **Blockchain**: Ethereum Testnet or Cardano (planned)

## Build Commands

```bash
# Build all packages
go build ./...

# Run tests
go test ./...

# Run specific test package with verbose output
go test ./internal/fantasy/sleeper/... -v

# Start API Gateway
go run cmd/gateway/main.go
```

## Project Structure

```
cmd/
└── gateway/
    └── main.go              # API Gateway server (Chi router, port 8080)

internal/
└── fantasy/
    └── sleeper/
        ├── client.go        # Sleeper API HTTP client
        ├── client_test.go   # Integration tests
        └── models.go        # Data structures (User, League, Roster, Team)
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/sleeper/user/{username}` | Lookup Sleeper user by username |
| GET | `/api/sleeper/user/{userId}/leagues` | Get user's NFL leagues (query: ?season=2024) |
| GET | `/api/sleeper/league/{leagueId}` | Get league details |
| GET | `/api/sleeper/league/{leagueId}/teams` | Get teams with rosters and owner info |
| GET | `/health` | Health check |

## Architecture

### Current Implementation

- **API Gateway** (`cmd/gateway`) - Chi-based HTTP server handling routing and middleware
- **Sleeper Client** (`internal/fantasy/sleeper`) - HTTP client for Sleeper Fantasy API integration

### Planned Services

| Service | Responsibility |
|---------|----------------|
| Auth Service | JWT authentication, user management |
| League Service | League CRUD operations |
| Payment Service | Entry fee processing |
| Oracle Service | Score fetching from fantasy platforms |
| Contract Manager | Blockchain interactions |

### External APIs

- **Sleeper API** (`https://api.sleeper.app/v1`) - Public API, no auth required, 1000 req/min limit
