# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WAGR is a Web3 application for managing payments for Fantasy sports leagues. Users can create leagues, connect wallets, and handle entry fees and payouts via smart contracts on blockchain (Ethereum Testnet/Cardano).

## Technology Stack

- **Frontend**: React/Next.js web application
- **Backend**: Go microservices
- **Database**: PostgreSQL with Redis caching
- **Blockchain**: Ethereum Testnet or Cardano with Solidity/smart contracts
- **Monitoring**: Prometheus/Grafana with Discord/Email alerts

## Build Commands

*Commands will be added as services are implemented. Expected patterns:*

**Go Services:**
- `go build ./...` - Build all services
- `go test ./...` - Run all tests
- `go run cmd/<service>/main.go` - Run a specific service

**Frontend:**
- `npm install` - Install dependencies
- `npm run dev` - Run development server
- `npm run build` - Production build

## Architecture

### Service Overview

| Service | Port | Responsibility |
|---------|------|----------------|
| API Gateway | 8080 | Routes requests to core services |
| Auth Service | - | JWT authentication, user management |
| League Service | - | League CRUD operations |
| Payment Service | - | Entry fee processing |
| Oracle Service | - | Score fetching from fantasy platforms |
| Contract Manager | - | Blockchain interactions |

### Core Layers

1. **Frontend Layer** - React/Next.js app with wallet connection (MetaMask, WalletConnect)
2. **API Layer** - Go services behind API Gateway (Auth, League, Payment)
3. **Background Services** - Oracle workers (ESPN, Yahoo, Sleeper), Cron Scheduler, Contract Manager
4. **Data Layer** - PostgreSQL (users, leagues, members, payouts, transactions, scores) + Redis cache
5. **Blockchain Layer** - Smart contracts: League Factory (deploys new leagues), League Contract (per-league instance), Payout Contract (distribution rules)

### Data Flow

- Frontend connects to API Gateway (port 8080) for all backend operations
- Frontend connects directly to blockchain for wallet/contract interactions
- Oracle workers fetch scores from ESPN/Yahoo/Sleeper APIs on schedule
- Contract Manager handles all smart contract deployments and interactions
- League and Oracle services communicate with Contract Manager for blockchain operations
