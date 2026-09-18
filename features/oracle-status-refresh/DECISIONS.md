# Feature: Oracle League Status Refresh

**Slug:** oracle-status-refresh
**Status:** Done
**Created:** 2026-09-16
**Source:** https://trello.com/c/CxjnC95d/6-p1-oracle-refresh-league-status-predraft-%E2%86%92-inseason

## What & Why

A league's `status` is only set at import (`src/internal/league/service.go:130`).
After that, the only code that changes it is the season-end job, which moves
`in_season` to `complete`. A league imported before its draft stays at `pre_draft`
forever. Both oracle jobs (`RunWeeklyPayoutJob`, `RunSeasonEndPayoutJob` in
`src/internal/league/service_oracle.go`) only pick up leagues with
`status = 'in_season'`, so they skip it. That means no weekly bonuses and no
season-end payouts. Right now the only workaround is to re-import the league.

This feature adds a status-refresh step to the oracle. It fetches each league's
current status from the platform and moves `leagues.status` forward before the
payout jobs run.

**Trigger / entry point:** the oracle binary (`src/cmd/oracle/main.go`). The
refresh runs at the start of **every** run, whether that's `-job=weekly`,
`-job=season` or no flag, before any payout job. In prod, weekly and season are
separate Render cron jobs (`infra/render.tf`), so this puts the refresh in front
of both without touching the infra.

## Scope

**In scope:**
- A new service method (e.g. `RefreshLeagueStatuses(ctx)`) in
  `src/internal/league/service_oracle.go`. It loads all leagues where
  `cancelled_at IS NULL` and `status` is not `complete`/`in_season` (i.e.
  `pre_draft`, `drafting`, or NULL), then calls
  `platformService.GetLeague(...)` for each one and updates `leagues.status`
  (and `updated_at`) when the transition is allowed.
- Transition rules, forward only, with the highest allowed state being `in_season`:
  - rank: `pre_draft` (and NULL) < `drafting` < `in_season` = `post_season` < `complete`
  - `post_season` (a real Sleeper status, missed in the first pass) is treated as a
    payable, not-yet-complete season: a platform `post_season` advances a
    `pre_draft`/`drafting` league to `in_season`, and a league *stored* as
    `post_season` (imported during the playoffs) is rewritten to `in_season` so the
    payout jobs, which query only for `in_season`, can see it
  - `complete` outranks `in_season` and is never walked back
  - update only when the platform's status ranks higher than the stored status
  - if the platform reports `complete` while the stored status is `pre_draft` or
    `drafting`, write `in_season`. Then the season-end job will pick the league
    up, see `complete` on the platform, and run placement payouts as usual
  - platform status that isn't recognized → no change, log it
- Only current-season leagues are refreshed. A league whose `season` isn't the
  platform's current season is skipped, so a stale prior-season league can't be
  advanced to `in_season` and then paid weekly bonuses against the current season's
  week numbering (`RunWeeklyPayoutJob` derives the week from Sleeper's *global* NFL
  state). This needs a new `GetCurrentSeason` method on the `FantasyPlatform`
  interface, since only `GetCurrentWeek` existed. Sleeper's `NFLState.Season` is
  used rather than `time.Now().Year()`, which is wrong during January/February
  playoffs.
- Call the refresh from `main.go` before the weekly and season jobs, whatever
  `-job` value was passed, on its own sub-budget (a quarter of `ORACLE_JOB_TIMEOUT`)
  so a slow platform can't consume the whole timeout and starve the payout jobs.
- Table-driven unit tests for the transition rules, plus a test of the refresh
  loop that uses a fake platform (per-league failures are skipped, `complete`
  is never written, nothing moves backward).

**Explicitly out of scope:**
- Writing `complete`. That stays with `processSeasonEndForLeague` because it is
  tied to running payouts.
- Moving status backward (e.g. `in_season` → `pre_draft` if Sleeper resets a
  league).
- Refreshing leagues that are already `in_season`. The season job already checks
  them for completion.
- A standalone `-job=status` flag or a new Render cron resource.
- Refreshing status when a user loads a league page or calls any API. No
  API/handler changes.
- Frontend changes. The existing status badges show the new value as-is.
- ESPN/Yahoo. The code goes through the platform-agnostic `PlatformService`, but
  only Sleeper is registered.
- Refreshing other league metadata (name, total_rosters, settings).

**Additive or replacing existing behavior:** Additive. The existing jobs and
their queries stay the same. The re-import workaround still works but is no
longer needed.

## Edge Cases & Known Limitations

- **Platform failure for one league** (timeout, 404, deleted league, rate limit):
  log with league ID, skip, continue with the next league. The refresh step never
  causes a non-zero exit, and the weekly/season jobs still run.
- **Refresh step fails as a whole** (e.g. the DB query to load leagues fails): log
  it and continue into the payout jobs. The refresh doesn't set the `failed` flag,
  so it can't block payouts. (Payout jobs still set `failed` on their own errors
  as they do today.)
- **Platform reports `complete` for a stored `pre_draft`/`drafting` league:**
  written as `in_season` only. On the season run, the league becomes `complete` and
  gets paid out, in that same invocation if the season job runs after the refresh.
  Otherwise it happens on the next season run.
- **`drafting` leagues** are stored as `drafting` and still skipped by both payout
  jobs. That's correct, since no games have been played.
- **Concurrency:** the weekly and season crons could overlap. The UPDATE is
  conditional (`WHERE id = $1 AND status IS NOT DISTINCT FROM $old`), which
  makes a double write harmless. Running it twice changes nothing.
- **Cancelled leagues** (`cancelled_at IS NOT NULL`) are never refreshed.
- **Known limitation:** one `GetLeague` call per non-started league per oracle
  run. Calls go one at a time with no batching. This is fine at current scale,
  well under Sleeper's 1000 req/min. The call count is bounded by
  `ORACLE_JOB_TIMEOUT`, and the refresh takes time out of the same budget as the
  payout jobs.
- **Known limitation:** status only updates when the oracle runs (cron cadence),
  not in real time.
- **Known limitation (pre-existing, separate card):** `processSeasonEndForLeague`
  writes `status = 'complete'` *before* its escrow check. A league whose escrow was
  never funded is therefore left `complete` + `payout_status = 'pending'`, and the
  season query only selects `in_season`, so it is never retried even once escrow is
  funded. This feature doesn't create the bug but does route more leagues through
  that path. Out of scope here; needs the status write moved after the escrow check.
- **Stale-season leagues** are skipped entirely by the refresh, so they stay stuck at
  `pre_draft` and are never paid. That is deliberate: paying them against the current
  season's weeks would be wrong.
- **Empty/missing platform status** is treated as unrecognized and logged, rather
  than silently ignored, so an adapter that stops populating `Status` is visible.

## Affected Systems

- Modules/services touched:
  - `src/internal/league/service_oracle.go`: new refresh method + transition helper
  - `src/cmd/oracle/main.go`: invoke refresh before the jobs, on a sub-budget
  - `src/internal/fantasy/platform.go`, `registry.go`, `sleeper/adapter.go`: new
    `GetCurrentSeason` method (Sleeper is the only implementation)
  - new test file in `src/internal/league/` for the refresh logic
- Data model changes: None. Uses the existing `leagues.status` and `updated_at`
  columns. No migration.
- Downstream risk:
  - More leagues become `in_season`, so the weekly and season-end payout jobs
    will start paying out on leagues that were stuck. **That is the goal, but it
    means real on-chain payouts start for those leagues.**
  - For a league that was stuck, the weekly job pays only the last completed week
    (`currentWeek - 1`). Earlier missed weeks are not backfilled. This is existing
    behavior and stays out of scope.
  - Frontend status badges will show the new status. No breaking change.

## Acceptance Criteria

- [x] With a league stored as `pre_draft` whose platform status is `in_season`,
      running `oracle -job=weekly` (and separately `-job=season`) updates
      `leagues.status` to `in_season` before that job's league query runs, so the
      job processes the league in the same run.
- [x] A league stored as `pre_draft` or `drafting` whose platform status is
      `complete` is updated to `in_season`, never to `complete`. After the
      season job runs in the same invocation, it goes through the existing
      `processSeasonEndForLeague` path (status `complete`, payouts executed).
- [x] Status never moves backward. A league stored as `drafting` whose platform
      reports `pre_draft` stays `drafting`. Leagues stored as `in_season`,
      `complete`, or with `cancelled_at` set are not queried against the platform.
- [x] If `GetLeague` returns an error for one league, a log line names the league
      ID, the other leagues still get refreshed, the payout jobs still run, and the
      process exit code doesn't change because of it.
- [x] A league whose `season` isn't the platform's current season is never advanced,
      and a platform whose `GetCurrentSeason` lookup fails has all its leagues skipped.
- [x] A league stored as `post_season`, and a `pre_draft`/`drafting` league whose
      platform reports `post_season`, both end up `in_season` so the payout jobs see
      them. A league stored as `complete` is never walked back to `in_season`.
- [x] Unit tests cover the transition table (every stored × platform status pair,
      including NULL and unknown values) and the refresh loop with a fake
      platform. `go test ./src/internal/league/...` passes.

## Risk Flags

- **Money-touching (indirect):** this change sends nothing on-chain itself, but
  changing the status is what lets leagues into the weekly and season-end payout
  jobs, which call `distributePayout` on Hedera. A bug that moves a league to
  `in_season` too early, or writes `complete` and skips the season job's own
  transition, would cause wrong payouts or skipped ones. Review must confirm that
  `complete` is never written and nothing moves backward.
- Irreversible: on-chain payouts that run because of this change can't be
  reversed.

## Implementation Log
### 2026-09-16
- Implemented: `nextLeagueStatus` (forward-only transition table, capped at
  `in_season`; platform `complete` → `in_season`), `refreshLeagueStatuses` (loop
  with injected fetch/update funcs), and `Service.RefreshLeagueStatuses` in
  `src/internal/league/service_oracle.go`. `src/cmd/oracle/main.go` calls it before
  the weekly/season jobs regardless of `-job`; errors are logged only.
  Tests in `src/internal/league/service_oracle_status_test.go`.
- Subagents used: `reviewer` (read-only, mandatory for the money-touching flag):
  approved, no blocking findings. Implementation and tests done inline (small change).
- Deviations from plan:
  - Conditional UPDATE uses `COALESCE(status, '') = $old` (NULL scanned as `""`)
    rather than `IS NOT DISTINCT FROM`. Equivalent given no code writes `''` to
    `leagues.status`.
  - Refresh loop takes fetch/update funcs as parameters so it can be unit-tested
    without a DB, since `Service` holds a concrete pgx pool and PlatformService.
  - Reviewer note: the refresh and season-job queries are disjoint by status, so the
    conditional UPDATE mainly guards against overlapping weekly+season cron refreshes.
- Acceptance criteria status: all met. AC 1, 2 and 4 are verified by code review and
  unit tests of the loop, not by an end-to-end oracle run against a live DB/Sleeper.

### 2026-09-18
- `/code-review` on the working tree found 6 issues the first reviewer pass missed,
  because that pass checked the code against this spec and the spec itself had the
  gaps. Fixed in this pass, with scope changes confirmed by the user first:
  - **post_season was missing entirely** (HIGH). Sleeper reports it and the frontend
    already renders it (`Home.tsx:172`); the transition table was written from the
    stale comment in `fantasy/models.go`. Playoff-stage leagues were treated as
    "unrecognized" and left stuck — the exact bug this feature exists to fix.
  - **No season filter** (MEDIUM, money-touching). Added `GetCurrentSeason` to the
    platform interface + `filterCurrentSeasonLeagues`.
  - **Shared job timeout** (LOW) contradicted the "never blocks payouts" guarantee;
    the refresh now gets its own sub-context.
  - **Discarded UPDATE row count** (LOW): `updateLeagueStatusFunc` now returns
    `(bool, error)` so a contended no-op is logged as such, not as a transition.
  - **Empty platform status** (LOW) no longer passes the unrecognized-status guard.
  - The stranded-unfunded-league bug is documented above as a separate card.
- While fixing post_season, the new tests caught a bug I introduced: rewriting any
  status whose canonical form differed sent a league stored as `complete` back to
  `in_season`, which would re-open a finished league and re-trigger payouts. The
  rewrite is now restricted to `post_season` only.
- Subagents used: `reviewer` (twice, read-only, mandatory for the money flag).
- Acceptance criteria status: all met at the unit level. Still not verified end to
  end against a live DB + Sleeper.
- Second reviewer pass: approved, no blocking findings. Verified post_season cannot
  leak into a `complete` write, the season guard has no default-to-current fallback,
  and the refresh sub-context can't shorten the parent or affect the exit code. Its
  one minor note (a failing `GetCurrentSeason` retried per league) fixed by tracking
  attempted platforms.
- Verified against a real Postgres 16 (scratch DB `wagr_refresh_scratch`, all 8
  migrations applied, seeded with 10 leagues covering every case, a fake platform
  adapter in place of Sleeper). All 10 landed on the expected status, and a second
  run changed nothing. Confirms the live SQL, not just the unit-level logic:
  the `post_season` filter, the new `season` column in the select, the NULL-status
  `COALESCE`, the cancelled-league exclusion and the `RowsAffected` check.
  The scratch DB and its temporary test file were deleted afterwards; the dev
  database was not written to.
- Stale status comments corrected in `src/internal/fantasy/models.go:22` and
  migration 004 (both listed only pre_draft/in_season/complete, the stale source
  that caused the post_season miss).
- Follow-up card filed for the stranded-unfunded-league bug:
  https://trello.com/c/HtDZdm97
