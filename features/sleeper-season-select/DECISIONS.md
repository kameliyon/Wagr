# Feature: Sleeper Season Selector

**Slug:** sleeper-season-select
**Status:** Done
**Created:** 2026-07-20

## What & Why

Today, `ImportLeagueModal.tsx` hardcodes `season=2025` when fetching a Sleeper
user's leagues (`ImportLeagueModal.tsx:63`), and the step 2 copy separately
hardcodes "2024 NFL season" (`ImportLeagueModal.tsx:173`) — the two are already
inconsistent with each other and with the current date. With the 2026 NFL
season starting soon, users need to pick which season to import leagues from
instead of being stuck on a stale hardcoded year.

Trigger / entry point: step 1 of the existing Import League modal (the
username-entry screen), where the user connects their Sleeper account.

The backend already supports this generically — `GetUserLeagues` in
`src/internal/fantasy/sleeper/client.go`, the `FantasyPlatform` interface in
`src/internal/fantasy/platform.go`, the registry in
`src/internal/fantasy/registry.go`, and the handler in
`src/internal/fantasy/handlers.go` (which defaults `season` to the current
year when omitted) all already accept `season` as a plain parameter. This is
a frontend-only fix.

## Scope

**In scope:**
- Add a Season `<select>` dropdown to step 1 of `ImportLeagueModal.tsx`,
  positioned next to a Platform field slot.
- Populate the dropdown with exactly 3 options: current year, current year
  − 1, current year − 2 (e.g. in 2026: 2026, 2025, 2024), derived from
  `new Date().getFullYear()` rather than hardcoded numbers.
- Default the dropdown to the current year (2026).
- Use the selected season in the `GET /api/fantasy/sleeper/user/{id}/leagues`
  fetch (replacing the hardcoded `season=2025`).
- Fix the step 2 copy to reference the actually-selected season instead of
  the hardcoded "2024 NFL season" string.

**Explicitly out of scope:**
- The Platform dropdown itself (Sleeper/ESPN/Yahoo selection, with ESPN/Yahoo
  disabled) — that's a separate feature, spec'd independently. This feature
  only reserves the layout slot next to it (see below) but keeps Platform
  fixed to "Sleeper" (not an interactive control).
- Sport selection — `sport=nfl` stays hardcoded.
- Any backend changes — the API already accepts season generically; no
  server-side range validation is being added in this pass.
- Auto-refetching leagues the instant the season dropdown changes while still
  on step 1 — refetch only happens when the user (re)submits step 1.

**Additive or replacing existing behavior:** Replacing — this replaces the
existing hardcoded season value and hardcoded step-2 copy with a
user-selected value threaded through the existing fetch calls.

## Edge Cases & Known Limitations

- If the user goes Back from step 2 to step 1, changes the season, and hits
  Connect again, step 1's submit handler re-runs `link-platform` and
  re-fetches leagues for the newly selected season, replacing
  `availableLeagues`. No client-side caching of prior-season results.
- If no leagues exist for the selected account/season, the existing "No
  leagues found for this account." message in step 2 covers it — no new
  empty-state copy needed.
- Layout reserves a slot for the future Platform dropdown (fixed/disabled,
  labeled "Sleeper") next to the Season dropdown, to avoid rework when the
  platform-selector feature lands, but does not make Platform interactive.
- Sport stays hardcoded to NFL for this pass; no sport selector.
- No concurrency concerns — this is a client-side selection feeding an
  existing read-only GET request.

## Affected Systems

- Modules/services touched: `src/web/src/components/ImportLeagueModal.tsx`
  only.
- Data model changes: None.
- Downstream risk: None — no backend, database, or contract changes; no
  other consumers of the modal's internal state.

## Acceptance Criteria

- [ ] Step 1 shows a Season dropdown next to a fixed/disabled Platform field
      (labeled "Sleeper"), defaulting to the current year (2026).
- [ ] Dropdown options are exactly the current year and the prior 2 years
      (2026, 2025, 2024 as of today), computed from the current date rather
      than hardcoded.
- [ ] Submitting step 1 fetches leagues using the selected season instead of
      the hardcoded `season=2025`.
- [ ] Step 2's descriptive text reflects the actually-selected season
      (fixing today's hardcoded "2024 NFL season" text).
- [ ] Going back to step 1, changing the season, and reconnecting re-fetches
      and displays leagues for the newly selected season.

## Risk Flags

None — no money movement, no auth/security surface. Importing a
wrong-season league is user-correctable (delete and re-import).

## Implementation Log

### 2026-07-21
- Implemented: Added a `SEASON_OPTIONS` constant (current year + prior 2,
  derived from `new Date().getFullYear()`) to `ImportLeagueModal.tsx`. Step 1
  now shows a Platform/Season row: Platform is a disabled `<select>` fixed to
  "Sleeper" (reserved slot for the future platform-selector feature), Season
  is an interactive `<select>` defaulting to the current year. The league
  fetch now uses the selected `season` state instead of the hardcoded
  `season=2025`, and step 2's copy now interpolates the selected season
  instead of the hardcoded "2024 NFL season" string. Added matching CSS for
  the field row and `<select>` styling in `ImportLeagueModal.css`.
- Subagents used: None — this was a single-file (plus its co-located CSS)
  change, handled directly per the skill's guidance to skip delegation for
  small, non-splittable work.
- Deviations from plan: None.
- Acceptance criteria status:
  - [x] Step 1 shows a Season dropdown next to a fixed/disabled Platform
        field, defaulting to the current year.
  - [x] Dropdown options are exactly the current year and prior 2 years,
        computed from the current date.
  - [x] Submitting step 1 fetches leagues using the selected season.
  - [x] Step 2's copy reflects the actually-selected season.
  - [x] Changing season after Back and reconnecting re-fetches for the new
        season.
  - Verified via a new test file, `src/test/ImportLeagueModal.season.test.tsx`
    (rendered-component tests using the project's existing
    testing-library/vitest conventions), plus `tsc --noEmit`. Full
    browser-driven verification wasn't possible because both entry points to
    this modal (`Home.tsx`, `Leagues.tsx`) sit behind wallet authentication,
    which requires a real Hedera wallet connection unavailable in this
    environment. Confirmed the 5 pre-existing failures in
    `LeagueSettings.bonus.test.tsx` predate this change (reproduced on a
    stashed clean checkout) and are unrelated.

### 2026-07-21 (follow-up)
- Implemented: Per user request, upgraded the Platform field from a fully
  disabled select to an enabled one with ESPN and Yahoo present as disabled
  options labeled "(Coming soon)"; Sleeper remains the only selectable value.
  Small enough to do inline rather than spinning up the separate
  platform-selector feature spec discussed earlier.
- Verified: `tsc --noEmit` clean, existing season test suite still passes.

