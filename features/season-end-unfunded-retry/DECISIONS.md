# Feature: Season-end job retries unfunded leagues

**Slug:** season-end-unfunded-retry
**Status:** Done
**Created:** 2026-09-19
**Source:** Trello card #15, "[P1] Season-end job strands unfunded leagues as complete + pending"

## What & Why

`processSeasonEndForLeague` (`src/internal/league/service_oracle.go`) writes
`status = 'complete'` as soon as the platform reports `complete`, before it has
loaded standings, loaded members, read escrow, or executed the payout. If any later
step bails out, the league is left `status = 'complete'` + `payout_status = 'pending'`.
`RunSeasonEndPayoutJob` only selects `status = 'in_season' AND payout_status = 'pending'`,
so that league is never retried. That holds even after escrow is funded. Placement payouts
then have to be triggered by hand.

The trigger that matters most is `ErrInsufficientEscrow` (escrow not funded, or
funded after the first season-job run). Any of these errors after the status write
strands a league the same way:
`GetFinalStandings`, `GetLeagueMembers`, `readContractPayment`, and the ID/contract
encoding errors.

After this change, `complete` means one thing only: **the season is over and
placement payouts are settled** (`executed`). An unfunded league, or one that hit a
transient failure, stays `in_season` + `pending`, and the next season-job run picks
it up again.

**Entry point:** the scheduled oracle (`src/cmd/oracle`) calls `RunSeasonEndPayoutJob`.
There's no user-facing trigger.

## Scope

**In scope:**
1. **Reorder the season-end flow.** Nothing is written to `leagues` before the payout
   is settled. The new sequence:
   - Platform not `complete`: return (unchanged).
   - No placement rules, or no eligible placement targets: one UPDATE setting
     `status = 'complete'`, `payout_status = 'executed'`, `payouts_executed_at = NOW()`.
   - Standings/members/escrow-read/encoding error, or `ErrInsufficientEscrow`: return
     the error and **write nothing**. The league stays `in_season` + `pending` and is
     retried next run.
   - **Claim:** immediately before `ExecuteDistributePayout`, run a conditional
     `UPDATE leagues SET payout_status = 'executing' WHERE id = $1 AND payout_status = 'pending'`.
     If 0 rows are affected, another run already claimed the league, so log it and skip.
     Don't send the tx.
   - Tx fails: `payout_status = 'failed'`. `status` **stays `in_season`**. The league
     isn't retried (same as today). It needs manual follow-up.
   - Tx succeeds: one UPDATE setting `status = 'complete'`, `payout_status = 'executed'`,
     `payout_tx_hash`, `payouts_executed_at`, conditional on `payout_status = 'executing'`.
     Then update `league_members` payout records as today.
   - Post-tx DB update fails: log a line with the prefix `[oracle] MANUAL CHECK` that
     includes the league ID and tx hash. The league stays `executing`, is never retried,
     and can't be paid twice.
2. **Migration 010:**
   - Widen `chk_payout_status` to allow `'executing'` (drop and re-add the constraint).
   - Backfill: `UPDATE leagues SET status = 'in_season', updated_at = NOW()
     WHERE status = 'complete' AND payout_status = 'pending' AND cancelled_at IS NULL`.
3. **Testability refactor:** move the season-end sequencing behind small injected funcs
   (DB writes / claim, escrow read, tx execution), following the existing
   `refreshLeagueStatuses` pattern in `service_oracle.go`. Then write unit tests
   against it.
4. **Weekly path: verify only.** Add a test or a written note confirming that
   `processWeeklyPayoutsForLeague`'s write-before-check leaves events unexecuted
   and retryable when escrow is short. The weekly code doesn't change.
5. Update the "Known limitation (pre-existing, separate card)" bullet in
   `features/oracle-status-refresh/DECISIONS.md` to point at this feature as the fix.

**Explicitly out of scope:**
- Any smart contract change. In particular, a per-league "already paid" guard in
  `LeagueEscrow.distributePayout` needs a redeploy and belongs with the contract
  upgrade work.
- The double-pay window on the **weekly** path (tx succeeds, then marking events
  `executed_at` fails). Recorded as a known limitation below.
- Automatic retry of `failed` or `executing` leagues. Both stay manual.
- Alerting or notification infrastructure. A log line is the only signal.
- Admin UI or API to inspect or reset stuck leagues.
- Backfilling missed weekly payouts (existing behavior, unchanged).
- Frontend changes. Nothing in `src/web` reads `payout_status`.

**Additive or replacing existing behavior:** changes existing behavior.
`status = 'complete'` is now written only when the payout is settled, instead of when
the platform first reports `complete`. It adds one new `payout_status` value
(`executing`).

## Edge Cases & Known Limitations

- **Escrow unfunded or short:** league stays `in_season` + `pending`. Each season run
  re-fetches the platform league, standings, members and escrow balance, and returns
  `ErrInsufficientEscrow` until the league is funded. This repeats every run with no
  backoff. That's acceptable at current scale, and it adds to the per-run Sleeper calls
  (well under 1000 req/min).
- **Transient failure** (standings/members/escrow read): nothing written, retried next run.
- **Overlapping oracle runs:** the conditional claim (`pending → executing`) lets only
  one run send the tx. This also closes a race that exists today, where two overlapping
  season runs could both pay the same league.
- **Crash or DB failure between the tx and the final UPDATE:** the league stays
  `executing` and gets a `[oracle] MANUAL CHECK` log line. A human checks the tx on
  the mirror node and sets `executed` or `pending` by hand.
  **Known limitation:** if the process dies outright (not a returned error), there
  is no log line, and the only trace is a league sitting in `executing`.
- **Tx fails:** `in_season` + `failed`. The season job doesn't retry it (needs
  `pending`). Recovery is manual, same as today.
- **The weekly job keeps processing leagues past season end.** A league that stays
  `in_season` (unfunded, `failed`, or `executing`) is still selected by
  `RunWeeklyPayoutJob` for `currentWeek - 1`. Event inserts are `ON CONFLICT DO NOTHING`
  and unexecuted events are retried, so this is idempotent. It can still pay weekly
  events the league had queued if escrow later covers them, which is correct.
- **Known limitation (weekly path, not fixed here):** if the weekly tx succeeds but
  marking `weekly_payout_events.executed_at` fails, those events are retried and
  could be paid again. The contract caps a repeat payout at the league's escrow
  balance but doesn't prevent it.
- **Weekly path, verified (Scope item 4):** on `ErrInsufficientEscrow`,
  `processWeeklyPayoutsForLeague` returns before marking `executed_at`, so events
  stay unexecuted. **But retry is scoped to a single week.** `RunWeeklyPayoutJob`
  only processes `week = currentWeek - 1`, and the unexecuted-events query filters on
  `week = $2`. Once the platform week advances, a prior week's unexecuted events are
  never queried again. So an unfunded league loses that week's bonuses. This is
  existing behavior and not changed here. Needs its own card.
- **Migration numbering:** this migration is **010**, not 009. The unmerged
  `origin/wallet_support` branch already owns `009_add_evm_wallet_type.sql`, and the
  local dev DB has that 009 applied (goose version 9, users constraint `hedera`/`evm`).
  A second 009 would be skipped by goose on any DB that already recorded version 9.
- **Backfill caveat:** the migration can't tell a league that was genuinely unpaid
  from one paid out manually outside the oracle without updating `payout_status`.
  Before running migration 010 against a non-testnet DB, check that the stranded
  rows were actually unpaid.

## Affected Systems

- Modules/services touched:
  - `src/internal/league/service_oracle.go`: `processSeasonEndForLeague` reordered;
    sequencing moved behind injected funcs
  - new/extended test file in `src/internal/league/` (e.g. `service_oracle_season_test.go`)
  - `src/internal/database/migrations/010_*.sql`: new migration
  - `features/oracle-status-refresh/DECISIONS.md`: doc pointer
- Data model changes: `chk_payout_status` widened to include `'executing'`
  (additive). One-off data backfill of stranded leagues. No new columns.
- Downstream risk:
  - `RunSeasonEndPayoutJob` and `RunWeeklyPayoutJob` are the only readers of
    `payout_status`, and the frontend doesn't read it. The new `executing` value
    doesn't match `'pending'`, so both jobs skip it, which is the intent.
  - Frontend status badges: stranded leagues go back from "complete" to "in season"
    after the backfill, and unfunded leagues stay "in season" after the platform
    season ends.
  - **Real payouts:** backfilled leagues become eligible, so the next season run pays
    them out once escrow covers the payout.

## Acceptance Criteria

- [x] Given a league whose platform reports `complete` and whose escrow balance is
      below the total placement payout, a season run returns `ErrInsufficientEscrow`
      and leaves the row at `status = 'in_season'`, `payout_status = 'pending'`
      (no UPDATE issued, no tx sent). A later run, after escrow is funded,
      executes the payout and sets `status = 'complete'`, `payout_status = 'executed'`
      with a tx hash.
- [x] A standings fetch, members load, or escrow read error on a platform-complete
      league writes nothing to `leagues`, and the league is picked up on the next run.
- [x] `ExecuteDistributePayout` is only called after a conditional
      `pending → executing` claim affects exactly 1 row. When the claim affects 0 rows,
      no tx is sent.
- [x] When the tx fails, the row ends as `status = 'in_season'`, `payout_status = 'failed'`.
      When the tx succeeds but the final UPDATE fails, the row stays `executing`, and a
      log line containing `[oracle] MANUAL CHECK`, the league ID and the tx hash is written.
- [x] Leagues with no placement rules or no eligible targets end as
      `status = 'complete'`, `payout_status = 'executed'` in a single UPDATE.
- [x] Migration 010 applies cleanly on a DB at migration 008. After it runs,
      `payout_status = 'executing'` is accepted by the constraint, and every
      non-cancelled `complete` + `pending` row is `in_season` + `pending`.
- [x] A test or written note confirms weekly events stay unexecuted and retryable
      on `ErrInsufficientEscrow`.
- [x] `go build ./...` and `go test ./...` pass.

## Risk Flags

- **Money-touching:** changes when and whether on-chain placement payouts are sent,
  and adds a claim step whose purpose is preventing double payouts. `work-feature`
  must route this through the `reviewer` agent, focusing on every path from
  "platform complete" to `ExecuteDistributePayout` and on the claim's conditional WHERE.
- **Irreversible:** on-chain payouts can't be undone. The backfill migration makes
  stranded leagues payable on the next oracle run.
- **Data migration:** backfill rewrites `leagues.status` on existing rows. Verify
  the target rows before running it outside testnet.

## Implementation Log
### 2026-09-19
- Implemented:
  - `processSeasonEndForLeague` is now a thin wrapper around `settleSeasonEnd(ctx, league, seasonEndDeps)`.
    All platform, DB, Mirror Node and tx calls are injected funcs, wired in `(s *Service) seasonEndDeps()`.
  - Nothing is written to `leagues` before settlement. A conditional `pending -> executing` claim runs before
    `distribute`. `complete` is only written together with `executed`. Tx failure gives
    `in_season` + `failed`. A post-tx write failure logs `[oracle] MANUAL CHECK` with the league ID and tx hash.
  - Migration `010_payout_executing_status.sql`: widens `chk_payout_status` with `executing`, and backfills
    stranded `complete` + `pending` leagues to `in_season`.
  - Doc pointer added to `features/oracle-status-refresh/DECISIONS.md`.
- Subagents used:
  - `test-writer`: `service_oracle_season_test.go`, 15 tests / 19 cases covering the settlement paths.
  - `reviewer`: required pass for the money-touching risk flag. No blocking findings on the fund-moving
    path. It confirmed no route to `distribute` skips the claim, and no retry route leads to a second payout.
- Deviations from plan:
  - The escrow contract ID is now resolved inside the escrow-balance read, so a bad ID still fails before
    any write and doesn't block leagues with nothing to pay out.
  - An extra `MANUAL CHECK` log is written when the tx fails *and* marking the league `failed` also fails.
    The league stays `executing`, so it still can't be paid twice.
  - The weekly path is verified by a written note (Known Limitations), not a unit test, because
    `processWeeklyPayoutsForLeague` isn't injectable. Verification found the per-week retry gap noted above.
- Acceptance criteria status:
  - All met except the migration criterion. Migration 010 was reviewed (the constraint change is safe on
    existing data, the backfill WHERE clause matches the spec, and it runs in goose's per-migration
    transaction), but Docker isn't available in this environment, so it was **not applied against Postgres**.
    Apply it via `go run src/cmd/gateway/main.go` on a DB at 008 before merging.
- Follow-up on 2026-09-19 (same session): renumbered the migration from 009 to **010**.
  The local dev DB reported goose version 9 applied with no matching schema change in
  master; the cause is `origin/wallet_support`'s unmerged `009_add_evm_wallet_type.sql`,
  which that DB has applied. Keeping 009 would have meant goose silently skipping this
  migration, leaving `chk_payout_status` un-widened, so every payout claim would fail
  the constraint at runtime.
- Migration 010 applied to the dev DB at `192.168.50.79` on 2026-09-20 via
  `DATABASE_URL=... go run src/cmd/gateway/main.go` (goose reports version 10).
  Verified after: the constraint accepts `executing`, and the 2 stranded leagues
  (`Battle of Ohio`, `Fire Swamp Test League`, both with NULL `payout_tx_hash` and
  NULL `payouts_executed_at`, i.e. genuinely unpaid) are back to `in_season` + `pending`.
  0 leagues remain stranded. **Both are now payout-eligible on the next oracle run.**
- All acceptance criteria met.
