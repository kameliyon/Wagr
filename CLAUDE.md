# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WAGR is a Web3 application for managing payments for Fantasy sports leagues. Users can create leagues, connect wallets, and handle entry fees and payouts via smart contracts on blockchain (Ethereum Testnet/Cardano).

## Technology Stack

- **Frontend**: React with Vite, TypeScript
- **Backend**: Go microservices with Chi router
- **Database**: PostgreSQL (Docker Compose for local dev)
- **Blockchain**: Midnight Network (testnet)

## Build Commands

```bash
# Start PostgreSQL (required for backend)
docker-compose up -d

# Build all Go packages
go build ./...

# Run Go tests
go test ./...

# Start API Gateway (requires PostgreSQL running)
go run src/cmd/gateway/main.go

# Frontend commands (run from src/web/)
cd src/web
npm install
npm run dev      # Start dev server on :5173
npm run build    # Production build
```

## Project Structure

```
src/
├── cmd/
│   └── gateway/
│       └── main.go              # API Gateway server (Chi router, port 8080)
├── internal/
│   ├── auth/
│   │   ├── handlers.go          # Auth HTTP handlers
│   │   ├── service.go           # Auth business logic (JWT, signature verification)
│   │   ├── models.go            # User, Session, request/response types
│   │   └── context.go           # Context helpers for auth claims
│   ├── database/
│   │   ├── postgres.go          # PostgreSQL connection pool
│   │   └── migrations/
│   │       └── 001_users.sql    # Users table schema
│   ├── fantasy/
│   │   └── sleeper/
│   │       ├── client.go        # Sleeper API HTTP client
│   │       ├── handlers.go      # Sleeper HTTP handlers
│   │       └── models.go        # Data structures
│   └── handlers/
│       └── helpers.go           # Shared handler utilities
└── web/                         # React frontend
    ├── src/
    │   ├── components/          # ConnectWallet, Navbar
    │   ├── hooks/               # useWallet
    │   ├── pages/               # Home
    │   ├── providers/           # MidnightProvider
    │   └── types/               # TypeScript type definitions
    └── package.json

docker-compose.yml               # PostgreSQL for local development
```

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/nonce` | Get nonce for wallet to sign |
| POST | `/api/auth/verify` | Verify signature, return JWT |
| GET | `/api/auth/me` | Get current user (requires JWT) |
| POST | `/api/auth/logout` | Invalidate session |

### Sleeper Integration

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/sleeper/user/{username}` | Lookup Sleeper user by username |
| GET | `/api/sleeper/user/{userId}/leagues` | Get user's NFL leagues (query: ?season=2024) |
| GET | `/api/sleeper/league/{leagueId}` | Get league details |
| GET | `/api/sleeper/league/{leagueId}/teams` | Get teams with rosters and owner info |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |

## Architecture

### Current Implementation

- **API Gateway** (`src/cmd/gateway`) - Chi-based HTTP server handling routing and middleware
- **Auth Service** (`src/internal/auth`) - Wallet-based authentication with JWT
- **Database** (`src/internal/database`) - PostgreSQL connection pool
- **Sleeper Client** (`src/internal/fantasy/sleeper`) - HTTP client for Sleeper Fantasy API
- **Frontend** (`src/web`) - React app with Midnight wallet integration

### Auth Flow

1. User clicks "Connect Wallet" in frontend
2. Frontend calls `window.midnight.{wallet}.enable()` to connect
3. Frontend sends wallet address to `POST /api/auth/nonce`
4. Backend generates nonce, stores it, returns message to sign
5. Frontend asks wallet to sign the message
6. Frontend sends signature to `POST /api/auth/verify`
7. Backend verifies Ed25519 signature, creates JWT
8. Frontend stores JWT for subsequent requests

### Planned Services

| Service | Responsibility |
|---------|----------------|
| League Service | League CRUD operations |
| Payment Service | Entry fee processing |
| Oracle Service | Score fetching from fantasy platforms |
| Contract Manager | Midnight blockchain interactions |

### External APIs

- **Sleeper API** (`https://api.sleeper.app/v1`) - Public API, no auth required, 1000 req/min limit
- **Midnight Network** - Testnet for wallet authentication and future payments

## Git Workflow

- **Do not commit code** - Claude should not run `git commit` or `git push` unless explicitly asked by the user
