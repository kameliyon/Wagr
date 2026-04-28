# Contract Upgrade Patterns

Smart contracts are immutable once deployed — you cannot edit code that is already on-chain. This document covers the two main patterns for handling contract changes in WAGR and when to use each.

---

## Pattern 1 — Versioned Redeployment

The simplest approach. Deploy a new contract, point the app at it, and abandon the old one.

**When to use:**
- Testnet development (no real funds at risk)
- Early mainnet before a season starts (escrow is empty)
- Bug fixes that need to go out before any payments have been collected

**Steps:**

1. Make your changes to `LeagueEscrow.sol`
2. Redeploy:
   ```bash
   cd contracts
   HEDERA_OPERATOR_ID=0.0.12345 \
   HEDERA_OPERATOR_KEY=302e... \
   HEDERA_USDC_EVM_ADDRESS=0x0000000000000000000000000000000000456858 \
   node scripts/deploy.mjs
   ```
3. Update both `.env` files with the new contract ID:
   ```bash
   # root .env
   HEDERA_ESCROW_CONTRACT_ID=0.0.NEWNUMBER

   # src/web/.env
   VITE_HEDERA_ESCROW_CONTRACT_ID=0.0.NEWNUMBER
   ```
4. Restart backend and frontend

**Before abandoning the old contract**, drain any funds from it:
```
distributePayout(leagueId, [recipientAddresses], [amounts])
```
Call this for every league that had payments collected. The old contract remains on-chain forever but nothing will point to it once your env vars are updated.

---

## Pattern 2 — Proxy Pattern (Production)

A thin proxy contract sits in front of the implementation. Its address never changes, so users and the app always talk to the same contract ID. When you need to upgrade, you deploy a new implementation and point the proxy at it.

```
User / Backend
      │
      ▼
 ProxyContract  ← address never changes, stored in .env
      │  delegates all calls via delegatecall
      ▼
 LeagueEscrowV2  ← new implementation deployed on upgrade
```

**When to use:**
- Mainnet with real funds in escrow
- Mid-season upgrades where the contract address can't change
- Any situation where users have bookmarked or hardcoded the contract address

**How it works on Hedera:**

Hedera supports EVM-compatible proxy patterns. The standard approach is [OpenZeppelin's TransparentUpgradeableProxy](https://docs.openzeppelin.com/contracts/4.x/api/proxy) or ERC-1967.

The proxy stores the implementation address in a specific storage slot. An `upgrade(newImplementation)` function (owner-only) swaps it out. All other calls are forwarded via `delegatecall`, so the proxy's storage holds all the state (balances, payments, etc.) while the logic lives in the implementation.

**Key constraint:** Storage layout must be preserved across upgrades. If `LeagueEscrowV1` has:
```solidity
mapping(bytes32 => mapping(address => uint256)) public payments;   // slot 0
mapping(bytes32 => uint256) public leagueTotals;                    // slot 1
```
Then `LeagueEscrowV2` must keep those in the same slots and only append new variables after them. Reordering storage corrupts all existing data.

**WAGR is not using this pattern yet.** It is the right choice before mainnet launch.

---

## Mid-Season Emergency Procedure

If a critical bug is found while a season is active and funds are in escrow:

1. **Pause new payments** — remove or disable the Pay Entry Fee UI in the frontend immediately (feature flag or redeploy frontend with the button hidden)
2. **Drain the old contract** — call `distributePayout` to return all funds to members, or move them to a safe holding account
3. **Deploy the fixed contract** — using versioned redeployment
4. **Re-enable payments** — point the app at the new contract

If using the proxy pattern, step 2 is not needed since funds stay in the proxy's storage through the upgrade.

---

## Tracking Contract Versions

Keep a record of deployed contract IDs so you can always find where funds are:

| Version | Network | Contract ID | Deployed | Status |
|---|---|---|---|---|
| v1 | testnet | — | — | pending first deploy |

Update this table each time you deploy. The old contract IDs let you find historical transactions on HashScan even after the app has moved on.

---

## Verifying a Deployment

After any deploy, confirm it's working before routing traffic to it:

```bash
# Check the contract exists on HashScan
https://hashscan.io/testnet/contract/0.0.YOURNUMBER

# Confirm the USDC token is associated (look for an associateToken tx in history)

# Run a read-only call via Mirror Node to confirm the contract responds
curl -X POST https://testnet.mirrornode.hedera.com/api/v1/contracts/call \
  -H "Content-Type: application/json" \
  -d '{
    "block": "latest",
    "to": "0x000000000000000000000000000000000YOURNUMBER",
    "data": "0x",
    "estimate": false
  }'
```

A successful response (even an empty one) confirms the contract is live and reachable.
