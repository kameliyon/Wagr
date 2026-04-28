# WAGR — Deploy & Test Guide

This guide covers how to get WAGR running locally for end-to-end testing of the USDC entry fee payment flow on Hedera testnet.

---

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) — for PostgreSQL
- [Go 1.23+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [HashPack wallet](https://www.hashpack.app/) browser extension — connected to **testnet**
- A Hedera testnet account with test HBAR (from [Hedera Portal](https://portal.hedera.com))
- Testnet USDC in your HashPack wallet (see [Getting Testnet USDC](#getting-testnet-usdc))

---

## Step 1 — Environment Variables

Create a `.env` file at the repo root:

```bash
# Database (matches docker-compose defaults — no changes needed for local dev)
DB_HOST=localhost
DB_PORT=5432
DB_USER=wagr
DB_PASSWORD=wagr_dev_password
DB_NAME=wagr

# JWT signing secret — any random string works for local dev
JWT_SECRET=your-local-jwt-secret-here

# Hedera — testnet USDC token ID
HEDERA_USDC_TOKEN_ID=0.0.456858
HEDERA_NETWORK=testnet

# Filled in after Step 3 (contract deployment)
HEDERA_ESCROW_CONTRACT_ID=
```

Create a `.env` file in `src/web/`:

```bash
# WalletConnect Project ID — get one free at https://cloud.walletconnect.com
VITE_WALLETCONNECT_PROJECT_ID=your-walletconnect-project-id

# Filled in after Step 3 (contract deployment)
VITE_HEDERA_ESCROW_CONTRACT_ID=
VITE_HEDERA_USDC_TOKEN_ID=0.0.456858
```

---

## Step 2 — Start PostgreSQL

```bash
docker-compose up -d
```

Verify it's healthy:

```bash
docker-compose ps
# postgres should show "healthy"
```

The migrations in `src/internal/database/migrations/` run automatically on first start.

---

## Step 3 — Deploy the LeagueEscrow Contract

This step puts the smart contract on Hedera testnet. You only need to do this once (or whenever the contract changes).

**You will need:**
- Your Hedera testnet operator account ID (e.g. `0.0.12345`) — from [Hedera Portal](https://portal.hedera.com)
- Your operator private key (DER-encoded hex, starts with `302e...`)
- The USDC EVM address for testnet: `0x0000000000000000000000000000000000456858`

```bash
cd contracts
npm install

HEDERA_OPERATOR_ID=0.0.12345 \
HEDERA_OPERATOR_KEY=302e... \
HEDERA_USDC_EVM_ADDRESS=0x0000000000000000000000000000000000456858 \
HEDERA_NETWORK=testnet \
node scripts/deploy.mjs
```

On success you'll see:

```
✅ LeagueEscrow deployed!
Contract ID: 0.0.5555555

Add to your .env:
HEDERA_ESCROW_CONTRACT_ID=0.0.5555555
VITE_HEDERA_ESCROW_CONTRACT_ID=0.0.5555555
```

Copy those two values into your `.env` files (root and `src/web/`).

---

## Step 4 — Start the Backend

From the repo root (with your `.env` sourced):

```bash
# Source env vars
export $(cat .env | grep -v '^#' | xargs)

# Run the API gateway
go run src/cmd/gateway/main.go
```

The API will be available at `http://localhost:8080`. Verify with:

```bash
curl http://localhost:8080/health
# → ok
```

---

## Step 5 — Start the Frontend

```bash
cd src/web
npm install
npm run dev
```

The frontend will be available at `http://localhost:5173`.

---

## Step 6 — End-to-End Payment Test

1. Open `http://localhost:5173` in your browser
2. Click **Connect Wallet** and connect your HashPack testnet wallet
3. Sign the authentication message when prompted
4. Import a Sleeper league:
   - Click **Import League**
   - Enter your Sleeper username
   - Select a league and import it
5. Open the league and go to **Settings** — set an entry fee (e.g. `$5.00`)
6. Go back to the league overview — you should see the **Pay Entry Fee** panel
7. Click **Pay Entry Fee** — the button will step through three stages:
   - `Fetching payment details…` — backend returns contract ID + amount
   - `Approving $5.00 USDC in HashPack…` — two HashPack prompts appear (approve allowance, then contract call)
   - `Confirming payment on-chain…` — backend reads Mirror Node to verify, updates DB
8. On success the panel switches to a **Paid** badge with a HashScan link

**Verify on HashScan:**
- Open `https://hashscan.io/testnet/contract/0.0.5555555` (your contract ID)
- You should see two transactions: the allowance approval and the `payEntryFee` call
- The `EntryFeePaid` event should be visible under the contract call

---

## Getting Testnet USDC

Hedera testnet USDC (`0.0.456858`) can be obtained from the [Hedera testnet USDC faucet](https://faucet.hedera.com) or by swapping testnet HBAR on [SaucerSwap testnet](https://testnet.saucerswap.finance/).

Your HashPack wallet will need to associate with the USDC token first:
1. Open HashPack → Tokens → Manage Tokens
2. Search for token ID `0.0.456858` and associate it
3. Use the faucet or swap to get a balance

---

## Redeploying the Contract

If you change `LeagueEscrow.sol`, you must redeploy:

1. Run `deploy.mjs` again — you'll get a new contract ID
2. Update `HEDERA_ESCROW_CONTRACT_ID` in your root `.env`
3. Update `VITE_HEDERA_ESCROW_CONTRACT_ID` in `src/web/.env`
4. Restart the backend and frontend

> **Note:** The old contract remains on-chain. If any test funds were paid into it, call `distributePayout` on the old contract to recover them before abandoning it.

---

## Common Issues

| Symptom | Likely cause | Fix |
|---|---|---|
| `USDC association failed` on deploy | Wrong `HEDERA_USDC_EVM_ADDRESS` | Use `0x0000000000000000000000000000000000456858` for testnet |
| HashPack shows no prompt | `VITE_WALLETCONNECT_PROJECT_ID` missing | Add it to `src/web/.env` |
| `402 Payment Required` on confirm | Mirror Node hasn't indexed the tx yet | Wait 3–5 seconds and retry |
| `on-chain payment not found` | Contract ID mismatch between frontend and backend | Verify both `.env` files have the same contract ID |
| DB connection refused | PostgreSQL not running | `docker-compose up -d` |
| `JWT_SECRET` error on auth | Env var not set | Export your `.env` before running the backend |
