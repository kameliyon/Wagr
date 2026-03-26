# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WAGR is a Web3 application for managing payments for Fantasy sports leagues. Users can import leagues from fantasy platforms (Sleeper, ESPN, Yahoo), connect wallets, and handle entry fees and payouts via smart contracts on blockchain.

## Technology Stack

- **Frontend**: React with Vite, TypeScript
- **Backend**: Go microservices with Chi router
- **Database**: PostgreSQL (Docker Compose for local dev)
- **Blockchain**: Hedera (testnet)

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
│   │       ├── 001_users.sql                    # Users table schema
│   │       ├── 002_add_wallet_type.sql           # wallet_type column
│   │       ├── 003_create_league_tables.sql      # Deprecated placeholder
│   │       └── 004_platform_agnostic_leagues.sql # platform_profiles, leagues, league_members, platform_credentials
│   ├── fantasy/
│   │   ├── models.go            # Platform-agnostic types (PlatformUser, PlatformLeague, etc.)
│   │   ├── platform.go          # FantasyPlatform interface
│   │   ├── registry.go          # Registry & PlatformService managing multiple platforms
│   │   ├── handlers.go          # Platform-agnostic HTTP handlers
│   │   └── sleeper/
│   │       ├── client.go        # Sleeper API HTTP client
│   │       ├── adapter.go       # Adapter implementing FantasyPlatform interface
│   │       ├── models.go        # Sleeper-specific data structures
│   │       └── client_test.go
│   ├── league/
│   │   ├── models.go            # League, LeagueMember, PlatformProfile, PayoutEntry, LeagueSettings
│   │   ├── handlers.go          # League management HTTP handlers
│   │   └── service.go           # League business logic and DB queries
│   └── handlers/
│       └── helpers.go           # Shared handler utilities
└── web/                         # React frontend
    ├── src/
    │   ├── App.tsx              # Main routing (Home, Leagues, LeagueSettings)
    │   ├── components/
    │   │   ├── ConnectWallet.tsx
    │   │   ├── Navbar.tsx
    │   │   └── ImportLeagueModal.tsx  # Three-step league import flow
    │   ├── hooks/
    │   │   └── useWallet.ts
    │   ├── pages/
    │   │   ├── Home.tsx
    │   │   ├── Leagues.tsx           # List/manage imported leagues
    │   │   └── LeagueSettings.tsx    # Entry fees & payout configuration
    │   ├── providers/
    │   │   ├── WalletProvider.tsx
    │   │   └── WalletConfig.tsx
    │   ├── strategies/
    │   │   └── HederaStrategy.ts     # Hedera wallet integration
    │   ├── types/
    │   │   ├── wallet.d.ts
    │   │   ├── league.ts
    │   │   └── hedera.d.ts
    │   └── utils/
    │       └── walletConstants.ts
    └── package.json

docker-compose.yml               # PostgreSQL for local development
```

## API Route Groups

- **`/api/auth/*`** - Wallet-based authentication (nonce, verify, me, logout)
- **`/api/fantasy/{platform}/*`** - Platform-agnostic fantasy data (public): user lookup, leagues, members, rosters
- **`/api/leagues/*`** - League management (JWT required): import, list, settings, delete; settings update is commissioner-only
- **`/health`** - Health check

## Architecture

### Services

- **API Gateway** (`src/cmd/gateway`) - Chi-based HTTP server handling routing and middleware
- **Auth Service** (`src/internal/auth`) - Wallet-based authentication with JWT; supports Hedera wallets
- **Fantasy Registry** (`src/internal/fantasy`) - Platform-agnostic abstraction over fantasy providers; Sleeper implemented, ESPN/Yahoo stubbed
- **League Service** (`src/internal/league`) - League import, member management, entry fee and payout configuration
- **Database** (`src/internal/database`) - PostgreSQL connection pool with 4 migrations

### Auth Flow

1. User clicks "Connect Wallet" in frontend
2. Frontend calls the wallet API to connect
3. Frontend sends wallet address to `POST /api/auth/nonce`
4. Backend generates nonce, stores it, returns message to sign
5. Frontend asks wallet to sign the message
6. Frontend sends signature to `POST /api/auth/verify`
7. Backend verifies signature, creates JWT
8. Frontend stores JWT for subsequent requests

### League Import Flow

1. User opens ImportLeagueModal and enters their Sleeper username
2. Frontend calls `POST /api/leagues/link-platform` to associate the WAGR account with the Sleeper account
3. User selects a league from their available leagues
4. Frontend calls `POST /api/leagues/import` to import the selected league
5. League members are created for all roster owners with matching WAGR accounts

### Payout Structure

Stored as JSONB in `leagues.payout_structure`. Two types:
- **Placement**: Fixed prizes for 1st, 2nd, 3rd place, etc.
- **Weekly**: Recurring weekly bonuses (amount per week × number of weeks)

### Database Schema (Migration 004)

| Table | Description |
|-------|-------------|
| `platform_profiles` | Links WAGR users to fantasy platform accounts |
| `leagues` | Imported fantasy leagues (platform, season, entry fee, payouts) |
| `league_members` | Members with roster, stats, wallet address, payment status |
| `platform_credentials` | OAuth tokens/API keys for platforms requiring auth |

### External APIs

- **Sleeper API** (`https://api.sleeper.app/v1`) - Public API, no auth required, 1000 req/min limit
- **Hedera** - Testnet for wallet authentication and future payments

## Git Workflow

- **Do not commit code** - Claude should not run `git commit` or `git push` unless explicitly asked by the user
