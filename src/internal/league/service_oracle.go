package league

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"wagr/src/internal/fantasy"

	"golang.org/x/crypto/sha3"
)

// oracleLeague holds the fields needed by the oracle background jobs.
type oracleLeague struct {
	ID               string
	Platform         string
	PlatformLeagueID string
	PayoutStructure  []PayoutEntry
}

// encodeLeagueTotalsCall ABI-encodes a call to leagueTotals(bytes32).
func encodeLeagueTotalsCall(leagueId [32]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("leagueTotals(bytes32)"))
	selector := h.Sum(nil)[:4]
	buf := make([]byte, 36)
	copy(buf[:4], selector)
	copy(buf[4:], leagueId[:])
	return buf
}

// RunWeeklyPayoutJob processes weekly bonus payouts for all active in-season leagues.
// It resolves the last completed scoring week via the platform, applies each league's
// weekly payout rules against that week's matchup scores, and calls distributePayout
// on-chain for any qualifying teams that have not yet been paid.
func (s *Service) RunWeeklyPayoutJob(ctx context.Context) error {
	if s.hederaClient == nil {
		return ErrMissingOperatorKey
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, platform, platform_league_id, COALESCE(payout_structure, '[]'::jsonb)
		FROM leagues
		WHERE status = 'in_season'
		  AND cancelled_at IS NULL
		  AND payout_structure IS NOT NULL
		  AND jsonb_array_length(payout_structure) > 0
	`)
	if err != nil {
		return fmt.Errorf("failed to load leagues for weekly payout job: %w", err)
	}

	var leagues []oracleLeague
	for rows.Next() {
		var l oracleLeague
		var payoutJSON []byte
		if err := rows.Scan(&l.ID, &l.Platform, &l.PlatformLeagueID, &payoutJSON); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan league row: %w", err)
		}
		if err := json.Unmarshal(payoutJSON, &l.PayoutStructure); err != nil {
			log.Printf("[oracle] skipping league %s: failed to parse payout structure: %v", l.ID, err)
			continue
		}
		leagues = append(leagues, l)
	}
	rows.Close()

	for _, league := range leagues {
		var weeklyEntries []PayoutEntry
		for _, e := range league.PayoutStructure {
			if e.Type == "weekly" {
				weeklyEntries = append(weeklyEntries, e)
			}
		}
		if len(weeklyEntries) == 0 {
			continue
		}

		currentWeek, err := s.platformService.GetCurrentWeek(ctx, fantasy.PlatformType(league.Platform))
		if err != nil {
			log.Printf("[oracle] failed to get current week for league %s: %v", league.ID, err)
			continue
		}
		week := currentWeek - 1
		if week < 1 {
			continue
		}

		if err := s.processWeeklyPayoutsForLeague(ctx, league, weeklyEntries, week); err != nil {
			log.Printf("[oracle] weekly payout failed for league %s week %d: %v", league.ID, week, err)
		}
	}

	return nil
}

func (s *Service) processWeeklyPayoutsForLeague(ctx context.Context, league oracleLeague, entries []PayoutEntry, week int) error {
	matchups, err := s.platformService.GetLeagueMatchups(ctx, fantasy.PlatformType(league.Platform), league.PlatformLeagueID, week)
	if err != nil {
		return fmt.Errorf("failed to fetch matchups: %w", err)
	}

	// Exclude bye-week entries (matchup_id == 0)
	var activeMatchups []fantasy.PlatformMatchup
	for _, m := range matchups {
		if m.MatchupID > 0 {
			activeMatchups = append(activeMatchups, m)
		}
	}

	members, err := s.GetLeagueMembers(ctx, league.ID)
	if err != nil {
		return fmt.Errorf("failed to load members: %w", err)
	}
	memberByRoster := make(map[int]LeagueMember, len(members))
	for _, m := range members {
		memberByRoster[m.RosterID] = m
	}

	for _, entry := range entries {
		qualifiers := weeklyQualifiers(activeMatchups, entry)
		for _, m := range qualifiers {
			member, ok := memberByRoster[m.RosterID]
			if !ok || member.WalletAddress == "" {
				log.Printf("[oracle] skipping roster %d (%s): no WAGR member or no wallet", m.RosterID, entry.BonusType)
				continue
			}
			if _, err := s.db.Exec(ctx, `
				INSERT INTO weekly_payout_events
					(league_id, week, roster_id, platform_user_id, payout_type, points, amount_cents, wallet_address)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (league_id, week, roster_id, payout_type) DO NOTHING
			`, league.ID, week, m.RosterID, member.PlatformUserID, entry.BonusType, m.Points, entry.AmountCents, member.WalletAddress); err != nil {
				return fmt.Errorf("failed to record weekly event for roster %d: %w", m.RosterID, err)
			}
		}
	}

	// Load all unexecuted events for this league+week — includes any from prior failed runs
	type weeklyEvent struct {
		ID            string
		WalletAddress string
		AmountCents   int64
	}
	eventRows, err := s.db.Query(ctx, `
		SELECT id, wallet_address, amount_cents
		FROM weekly_payout_events
		WHERE league_id = $1 AND week = $2 AND executed_at IS NULL
	`, league.ID, week)
	if err != nil {
		return fmt.Errorf("failed to load unexecuted weekly events: %w", err)
	}

	var events []weeklyEvent
	for eventRows.Next() {
		var ev weeklyEvent
		if err := eventRows.Scan(&ev.ID, &ev.WalletAddress, &ev.AmountCents); err != nil {
			eventRows.Close()
			return fmt.Errorf("failed to scan weekly event: %w", err)
		}
		events = append(events, ev)
	}
	eventRows.Close()

	if len(events) == 0 {
		return nil
	}

	leagueIDBytes, err := uuidToBytes32(league.ID)
	if err != nil {
		return fmt.Errorf("invalid league ID: %w", err)
	}
	contractEVM, err := hederaAccountToEVM(s.hederaEscrowContractID)
	if err != nil {
		return fmt.Errorf("invalid escrow contract ID: %w", err)
	}

	var recipients [][20]byte
	var amounts []int64
	var executedEventIDs []string

	for _, ev := range events {
		addr, err := s.getAccountEVMAddress(ctx, ev.WalletAddress)
		if err != nil {
			log.Printf("[oracle] skipping wallet %s: failed to resolve EVM address: %v", ev.WalletAddress, err)
			continue
		}
		recipients = append(recipients, addr)
		amounts = append(amounts, ev.AmountCents*10_000)
		executedEventIDs = append(executedEventIDs, ev.ID)
	}

	if len(recipients) == 0 {
		return nil
	}

	var totalPayout int64
	for _, a := range amounts {
		totalPayout += a
	}
	escrowBalance, err := s.readContractPayment(ctx, contractEVM, encodeLeagueTotalsCall(leagueIDBytes))
	if err != nil {
		return fmt.Errorf("failed to read escrow balance: %w", err)
	}
	if totalPayout > escrowBalance {
		return ErrInsufficientEscrow
	}

	txHash, err := s.hederaClient.ExecuteDistributePayout(ctx, leagueIDBytes, recipients, amounts)
	if err != nil {
		return fmt.Errorf("on-chain execution failed: %w", err)
	}

	for _, id := range executedEventIDs {
		if _, err := s.db.Exec(ctx, `
			UPDATE weekly_payout_events
			SET tx_hash = $2, executed_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, id, txHash); err != nil {
			log.Printf("[oracle] failed to mark weekly event %s executed: %v", id, err)
		}
	}

	log.Printf("[oracle] weekly payouts for league %s week %d: %d recipients, tx %s", league.ID, week, len(recipients), txHash)
	return nil
}

// weeklyQualifiers returns the matchup entries that qualify for a given weekly payout rule.
func weeklyQualifiers(matchups []fantasy.PlatformMatchup, entry PayoutEntry) []fantasy.PlatformMatchup {
	switch entry.BonusType {
	case "weekly_high_score":
		return highScorer(matchups)
	case "score_threshold":
		if entry.Criteria == nil || entry.Criteria.Threshold == nil {
			return nil
		}
		return thresholdScorers(matchups, *entry.Criteria.Threshold)
	}
	return nil
}

// highScorer returns the single matchup with the highest points (skips 0-point entries).
func highScorer(matchups []fantasy.PlatformMatchup) []fantasy.PlatformMatchup {
	var best *fantasy.PlatformMatchup
	for i := range matchups {
		if matchups[i].Points <= 0 {
			continue
		}
		if best == nil || matchups[i].Points > best.Points {
			best = &matchups[i]
		}
	}
	if best == nil {
		return nil
	}
	return []fantasy.PlatformMatchup{*best}
}

// thresholdScorers returns all matchups where Points >= threshold.
func thresholdScorers(matchups []fantasy.PlatformMatchup, threshold float64) []fantasy.PlatformMatchup {
	var result []fantasy.PlatformMatchup
	for _, m := range matchups {
		if m.Points >= threshold {
			result = append(result, m)
		}
	}
	return result
}

// RunSeasonEndPayoutJob polls each in-season league's status on the platform.
// When a league transitions to "complete", it fetches the winners bracket standings
// and executes placement payouts on-chain.
func (s *Service) RunSeasonEndPayoutJob(ctx context.Context) error {
	if s.hederaClient == nil {
		return ErrMissingOperatorKey
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, platform, platform_league_id, COALESCE(payout_structure, '[]'::jsonb)
		FROM leagues
		WHERE status = 'in_season'
		  AND payout_status = 'pending'
		  AND cancelled_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to load leagues for season-end payout job: %w", err)
	}

	var leagues []oracleLeague
	for rows.Next() {
		var l oracleLeague
		var payoutJSON []byte
		if err := rows.Scan(&l.ID, &l.Platform, &l.PlatformLeagueID, &payoutJSON); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan league row: %w", err)
		}
		if err := json.Unmarshal(payoutJSON, &l.PayoutStructure); err != nil {
			log.Printf("[oracle] skipping league %s: failed to parse payout structure: %v", l.ID, err)
			continue
		}
		leagues = append(leagues, l)
	}
	rows.Close()

	for _, league := range leagues {
		if err := s.processSeasonEndForLeague(ctx, league); err != nil {
			log.Printf("[oracle] season-end payout failed for league %s: %v", league.ID, err)
		}
	}

	return nil
}

// seasonEndDeps are the external calls the season-end settlement makes, injected so
// the ordering of reads, the payout claim and the on-chain call can be unit tested.
type seasonEndDeps struct {
	platformStatus func(ctx context.Context, platform, platformLeagueID string) (string, error)
	finalStandings func(ctx context.Context, platform, platformLeagueID string) ([]fantasy.PlatformStanding, error)
	loadMembers    func(ctx context.Context, leagueID string) ([]LeagueMember, error)
	resolveEVM     func(ctx context.Context, accountID string) ([20]byte, error)
	escrowBalance  func(ctx context.Context, leagueID [32]byte) (int64, error)
	distribute     func(ctx context.Context, leagueID [32]byte, recipients [][20]byte, amounts []int64) (string, error)

	// markSettledWithoutPayout sets complete + executed for a league with nothing to pay.
	markSettledWithoutPayout func(ctx context.Context, leagueID string) error
	// claimPayout moves payout_status pending -> executing and reports whether this
	// run won the claim. Only the winner may send the payout transaction.
	claimPayout func(ctx context.Context, leagueID string) (bool, error)
	// markPayoutFailed moves payout_status executing -> failed. Status stays in_season.
	markPayoutFailed func(ctx context.Context, leagueID string) error
	// markPayoutExecuted sets complete + executed + tx hash, conditional on executing,
	// and reports whether a row was updated.
	markPayoutExecuted func(ctx context.Context, leagueID, txHash string) (bool, error)
	recordMemberPayout func(ctx context.Context, leagueID string, rosterID, place int, amountCents int64, txHash string) error
}

// seasonEndDeps wires settleSeasonEnd to the platform service, the database, the
// Mirror Node and the escrow contract.
func (s *Service) seasonEndDeps() seasonEndDeps {
	return seasonEndDeps{
		platformStatus: func(ctx context.Context, platform, platformLeagueID string) (string, error) {
			l, err := s.platformService.GetLeague(ctx, fantasy.PlatformType(platform), platformLeagueID)
			if err != nil {
				return "", err
			}
			return l.Status, nil
		},
		finalStandings: func(ctx context.Context, platform, platformLeagueID string) ([]fantasy.PlatformStanding, error) {
			return s.platformService.GetFinalStandings(ctx, fantasy.PlatformType(platform), platformLeagueID)
		},
		loadMembers: s.GetLeagueMembers,
		resolveEVM:  s.getAccountEVMAddress,
		escrowBalance: func(ctx context.Context, leagueID [32]byte) (int64, error) {
			contractEVM, err := hederaAccountToEVM(s.hederaEscrowContractID)
			if err != nil {
				return 0, fmt.Errorf("invalid escrow contract ID: %w", err)
			}
			return s.readContractPayment(ctx, contractEVM, encodeLeagueTotalsCall(leagueID))
		},
		distribute: s.hederaClient.ExecuteDistributePayout,
		markSettledWithoutPayout: func(ctx context.Context, leagueID string) error {
			_, err := s.db.Exec(ctx, `
				UPDATE leagues
				SET status = 'complete', payout_status = 'executed', payouts_executed_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND payout_status = 'pending'
			`, leagueID)
			return err
		},
		claimPayout: func(ctx context.Context, leagueID string) (bool, error) {
			tag, err := s.db.Exec(ctx, `
				UPDATE leagues SET payout_status = 'executing', updated_at = NOW()
				WHERE id = $1 AND payout_status = 'pending'
			`, leagueID)
			if err != nil {
				return false, err
			}
			return tag.RowsAffected() == 1, nil
		},
		markPayoutFailed: func(ctx context.Context, leagueID string) error {
			_, err := s.db.Exec(ctx, `
				UPDATE leagues SET payout_status = 'failed', updated_at = NOW()
				WHERE id = $1 AND payout_status = 'executing'
			`, leagueID)
			return err
		},
		markPayoutExecuted: func(ctx context.Context, leagueID, txHash string) (bool, error) {
			tag, err := s.db.Exec(ctx, `
				UPDATE leagues
				SET status = 'complete', payout_status = 'executed', payout_tx_hash = $2,
				    payouts_executed_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND payout_status = 'executing'
			`, leagueID, txHash)
			if err != nil {
				return false, err
			}
			return tag.RowsAffected() == 1, nil
		},
		recordMemberPayout: func(ctx context.Context, leagueID string, rosterID, place int, amountCents int64, txHash string) error {
			_, err := s.db.Exec(ctx, `
				UPDATE league_members
				SET final_rank = $1, payout_amount_cents = $2, payout_tx_hash = $3, payout_paid_at = NOW(), updated_at = NOW()
				WHERE league_id = $4 AND roster_id = $5
			`, place, amountCents, txHash, leagueID, rosterID)
			return err
		},
	}
}

func (s *Service) processSeasonEndForLeague(ctx context.Context, league oracleLeague) error {
	return settleSeasonEnd(ctx, league, s.seasonEndDeps())
}

// settleSeasonEnd pays out placement prizes for a league whose season is complete on
// the platform. Nothing is written to the league row until the payout is settled, so
// a league that can't be paid yet (escrow short, platform or Mirror Node error) stays
// in_season + pending and is retried on the next run. "complete" is only ever written
// together with payout_status = 'executed'.
func settleSeasonEnd(ctx context.Context, league oracleLeague, deps seasonEndDeps) error {
	platformStatus, err := deps.platformStatus(ctx, league.Platform, league.PlatformLeagueID)
	if err != nil {
		return fmt.Errorf("failed to fetch platform league status: %w", err)
	}
	if platformStatus != "complete" {
		return nil
	}

	var placementEntries []PayoutEntry
	for _, e := range league.PayoutStructure {
		if e.Type == "placement" {
			placementEntries = append(placementEntries, e)
		}
	}
	if len(placementEntries) == 0 {
		log.Printf("[oracle] league %s season complete with no placement rules; marking executed", league.ID)
		return deps.markSettledWithoutPayout(ctx, league.ID)
	}

	standings, err := deps.finalStandings(ctx, league.Platform, league.PlatformLeagueID)
	if err != nil {
		return fmt.Errorf("failed to fetch final standings: %w", err)
	}

	members, err := deps.loadMembers(ctx, league.ID)
	if err != nil {
		return fmt.Errorf("failed to load members: %w", err)
	}
	memberByRoster := make(map[int]LeagueMember, len(members))
	for _, m := range members {
		memberByRoster[m.RosterID] = m
	}

	placeToEntry := make(map[int]PayoutEntry, len(placementEntries))
	for _, e := range placementEntries {
		placeToEntry[e.Place] = e
	}

	type resolvedPayout struct {
		member      LeagueMember
		amountCents int64
		place       int
		evmAddr     [20]byte
	}

	var resolved []resolvedPayout
	for _, standing := range standings {
		entry, ok := placeToEntry[standing.Place]
		if !ok {
			continue
		}
		member, ok := memberByRoster[standing.RosterID]
		if !ok {
			log.Printf("[oracle] no member for roster %d (place %d) in league %s", standing.RosterID, standing.Place, league.ID)
			continue
		}
		if member.WalletAddress == "" {
			log.Printf("[oracle] skipping %s (place %d): no wallet address", member.DisplayName, standing.Place)
			continue
		}
		addr, err := deps.resolveEVM(ctx, member.WalletAddress)
		if err != nil {
			log.Printf("[oracle] skipping %s: failed to resolve EVM address: %v", member.WalletAddress, err)
			continue
		}
		resolved = append(resolved, resolvedPayout{
			member:      member,
			amountCents: entry.AmountCents,
			place:       standing.Place,
			evmAddr:     addr,
		})
	}

	if len(resolved) == 0 {
		log.Printf("[oracle] league %s: no eligible placement targets; marking executed", league.ID)
		return deps.markSettledWithoutPayout(ctx, league.ID)
	}

	leagueIDBytes, err := uuidToBytes32(league.ID)
	if err != nil {
		return fmt.Errorf("invalid league ID: %w", err)
	}

	recipients := make([][20]byte, len(resolved))
	amounts := make([]int64, len(resolved))
	var totalPayout int64
	for i, r := range resolved {
		recipients[i] = r.evmAddr
		amounts[i] = r.amountCents * 10_000 // cents -> 6-decimal micro-USDC
		totalPayout += amounts[i]
	}

	escrowBalance, err := deps.escrowBalance(ctx, leagueIDBytes)
	if err != nil {
		return fmt.Errorf("failed to read escrow balance: %w", err)
	}
	if totalPayout > escrowBalance {
		return ErrInsufficientEscrow
	}

	// Claim the league before sending the transaction. A league left in 'executing'
	// is never selected again, so a crash or DB failure after the transaction can't
	// lead to a second payout; it needs a manual check instead.
	claimed, err := deps.claimPayout(ctx, league.ID)
	if err != nil {
		return fmt.Errorf("failed to claim league for payout: %w", err)
	}
	if !claimed {
		log.Printf("[oracle] league %s no longer pending; another run claimed the payout first", league.ID)
		return nil
	}

	txHash, err := deps.distribute(ctx, leagueIDBytes, recipients, amounts)
	if err != nil {
		if dbErr := deps.markPayoutFailed(ctx, league.ID); dbErr != nil {
			log.Printf("[oracle] MANUAL CHECK: league %s payout tx failed and marking it failed also failed; left in 'executing': %v", league.ID, dbErr)
		}
		return fmt.Errorf("on-chain execution failed: %w", err)
	}

	updated, err := deps.markPayoutExecuted(ctx, league.ID, txHash)
	if err != nil || !updated {
		log.Printf("[oracle] MANUAL CHECK: league %s paid out in tx %s but was not marked executed (updated=%t, err=%v); left in 'executing'", league.ID, txHash, updated, err)
		if err != nil {
			return fmt.Errorf("failed to update league payout status: %w", err)
		}
		return fmt.Errorf("league %s was not in 'executing' when marking tx %s executed", league.ID, txHash)
	}

	for _, r := range resolved {
		if err := deps.recordMemberPayout(ctx, league.ID, r.member.RosterID, r.place, r.amountCents, txHash); err != nil {
			log.Printf("[oracle] failed to update member %s payout record: %v", r.member.DisplayName, err)
		}
	}

	log.Printf("[oracle] season-end payouts for league %s: %d recipients, tx %s", league.ID, len(resolved), txHash)
	return nil
}

// leagueStatusRank orders the statuses the status refresh understands.
// "in_season" is the highest status the refresh may ever write: "complete" is
// deliberately mapped down (see canonicalLeagueStatus) because only
// processSeasonEndForLeague may set it, and that transition is what triggers
// placement payouts.
var leagueStatusRank = map[string]int{
	"pre_draft":   0,
	"drafting":    1,
	"in_season":   2,
	"post_season": 2, // playoffs: still a payable, not-yet-complete season
	"complete":    3,
}

// canonicalLeagueStatus maps a platform-reported status onto the status the refresh
// stores. "post_season" and "complete" both become "in_season": the payout jobs only
// query for "in_season", and the season-end job is what recognizes completion and
// pays out placements.
func canonicalLeagueStatus(status string) string {
	switch status {
	case "post_season", "complete":
		return "in_season"
	default:
		return status
	}
}

// nextLeagueStatus returns the status to store given the stored and platform-reported
// statuses, and whether an update is needed. Transitions only move forward, and the
// highest status this can write is "in_season". A stored "" (NULL from import) is
// treated as "pre_draft".
func nextLeagueStatus(stored, platform string) (string, bool) {
	effectiveStored := stored
	if effectiveStored == "" {
		effectiveStored = "pre_draft" // NULL from import
	}
	storedRank, ok := leagueStatusRank[effectiveStored]
	if !ok {
		return stored, false
	}

	// A league stored as "post_season" is rewritten to "in_season" so the payout jobs
	// (which query only for "in_season") can see it. This is the one case where the
	// rank doesn't increase, so it's handled before the rank comparison. It only
	// applies to "post_season": "complete" outranks "in_season" and must never be
	// walked back, since that would re-open a finished league.
	if effectiveStored == "post_season" {
		if platformRank, ok := leagueStatusRank[platform]; ok && platformRank >= storedRank {
			return "in_season", true
		}
		return stored, false
	}

	target := canonicalLeagueStatus(platform)
	targetRank, ok := leagueStatusRank[target]
	if !ok || targetRank <= storedRank {
		return stored, false
	}
	return target, true
}

// statusRefreshLeague is a league eligible for status refresh.
type statusRefreshLeague struct {
	ID               string
	Platform         string
	PlatformLeagueID string
	Season           string
	Status           string // "" when NULL
}

// filterCurrentSeasonLeagues drops leagues that aren't part of their platform's
// current season, so a stale prior-season league can't be advanced to "in_season"
// and then paid weekly bonuses against the current season's week numbering.
// A platform missing from seasonByPlatform (its lookup failed) has all its leagues
// dropped: skipping a refresh is always safer than a wrong payout.
func filterCurrentSeasonLeagues(leagues []statusRefreshLeague, seasonByPlatform map[string]string) []statusRefreshLeague {
	var kept []statusRefreshLeague
	for _, l := range leagues {
		currentSeason, ok := seasonByPlatform[l.Platform]
		if !ok {
			log.Printf("[oracle] status refresh: skipping league %s: no current season for platform %s", l.ID, l.Platform)
			continue
		}
		if l.Season != currentSeason {
			log.Printf("[oracle] status refresh: skipping league %s: season %q is not the current %s season (%q)", l.ID, l.Season, l.Platform, currentSeason)
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

type fetchPlatformStatusFunc func(ctx context.Context, platform, platformLeagueID string) (string, error)

// updateLeagueStatusFunc applies the status change and reports whether a row was
// actually updated. A false return means the stored status changed underneath us
// (another oracle run got there first), which is not an error.
type updateLeagueStatusFunc func(ctx context.Context, leagueID, oldStatus, newStatus string) (bool, error)

// refreshLeagueStatuses applies nextLeagueStatus to each league, skipping any league
// whose platform lookup or update fails. Returns the number of leagues updated.
func refreshLeagueStatuses(ctx context.Context, leagues []statusRefreshLeague, fetch fetchPlatformStatusFunc, update updateLeagueStatusFunc) int {
	updated := 0
	for _, l := range leagues {
		platformStatus, err := fetch(ctx, l.Platform, l.PlatformLeagueID)
		if err != nil {
			log.Printf("[oracle] status refresh: failed to fetch platform status for league %s: %v", l.ID, err)
			continue
		}
		if _, known := leagueStatusRank[platformStatus]; !known {
			log.Printf("[oracle] status refresh: league %s has unrecognized platform status %q; leaving %q", l.ID, platformStatus, l.Status)
			continue
		}
		next, ok := nextLeagueStatus(l.Status, platformStatus)
		if !ok {
			continue
		}
		changed, err := update(ctx, l.ID, l.Status, next)
		if err != nil {
			log.Printf("[oracle] status refresh: failed to update league %s: %v", l.ID, err)
			continue
		}
		if !changed {
			log.Printf("[oracle] status refresh: league %s no longer %q; another run updated it first", l.ID, l.Status)
			continue
		}
		log.Printf("[oracle] status refresh: league %s %q -> %q (platform reports %q)", l.ID, l.Status, next, platformStatus)
		updated++
	}
	return updated
}

// RefreshLeagueStatuses advances the local status of leagues that haven't started
// (pre_draft, drafting, or NULL) plus leagues stored as post_season, based on the
// platform's current status, so the weekly and season-end payout jobs pick them up.
// Only current-season leagues are considered.
func (s *Service) RefreshLeagueStatuses(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, platform, platform_league_id, season, COALESCE(status, '')
		FROM leagues
		WHERE cancelled_at IS NULL
		  AND (status IS NULL OR status IN ('pre_draft', 'drafting', 'post_season'))
	`)
	if err != nil {
		return fmt.Errorf("failed to load leagues for status refresh: %w", err)
	}

	var leagues []statusRefreshLeague
	for rows.Next() {
		var l statusRefreshLeague
		if err := rows.Scan(&l.ID, &l.Platform, &l.PlatformLeagueID, &l.Season, &l.Status); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan league row: %w", err)
		}
		leagues = append(leagues, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate league rows: %w", err)
	}

	// Resolve each platform's current season once, not once per league. Failures are
	// recorded as attempted too, so one broken platform can't cause a lookup per league.
	seasonByPlatform := make(map[string]string)
	attempted := make(map[string]bool)
	for _, l := range leagues {
		if attempted[l.Platform] {
			continue
		}
		attempted[l.Platform] = true
		season, err := s.platformService.GetCurrentSeason(ctx, fantasy.PlatformType(l.Platform))
		if err != nil {
			log.Printf("[oracle] status refresh: failed to get current season for platform %s: %v", l.Platform, err)
			continue
		}
		seasonByPlatform[l.Platform] = season
	}

	candidates := filterCurrentSeasonLeagues(leagues, seasonByPlatform)

	fetch := func(ctx context.Context, platform, platformLeagueID string) (string, error) {
		pl, err := s.platformService.GetLeague(ctx, fantasy.PlatformType(platform), platformLeagueID)
		if err != nil {
			return "", err
		}
		return pl.Status, nil
	}
	update := func(ctx context.Context, leagueID, oldStatus, newStatus string) (bool, error) {
		// Conditional on the status we read, so overlapping oracle runs can't clobber
		// a newer transition (e.g. the season job setting "complete").
		tag, err := s.db.Exec(ctx, `
			UPDATE leagues SET status = $2, updated_at = NOW()
			WHERE id = $1 AND COALESCE(status, '') = $3
		`, leagueID, newStatus, oldStatus)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	}

	updated := refreshLeagueStatuses(ctx, candidates, fetch, update)
	log.Printf("[oracle] status refresh: %d eligible leagues, %d current-season, %d updated", len(leagues), len(candidates), updated)
	return nil
}
