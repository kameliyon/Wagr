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

func (s *Service) processSeasonEndForLeague(ctx context.Context, league oracleLeague) error {
	platformLeague, err := s.platformService.GetLeague(ctx, fantasy.PlatformType(league.Platform), league.PlatformLeagueID)
	if err != nil {
		return fmt.Errorf("failed to fetch platform league status: %w", err)
	}
	if platformLeague.Status != "complete" {
		return nil
	}

	if _, err := s.db.Exec(ctx, `
		UPDATE leagues SET status = 'complete', updated_at = NOW() WHERE id = $1
	`, league.ID); err != nil {
		return fmt.Errorf("failed to update local league status: %w", err)
	}

	var placementEntries []PayoutEntry
	for _, e := range league.PayoutStructure {
		if e.Type == "placement" {
			placementEntries = append(placementEntries, e)
		}
	}
	if len(placementEntries) == 0 {
		log.Printf("[oracle] league %s season complete with no placement rules; marking executed", league.ID)
		_, err = s.db.Exec(ctx, `
			UPDATE leagues SET payout_status = 'executed', payouts_executed_at = NOW(), updated_at = NOW() WHERE id = $1
		`, league.ID)
		return err
	}

	standings, err := s.platformService.GetFinalStandings(ctx, fantasy.PlatformType(league.Platform), league.PlatformLeagueID)
	if err != nil {
		return fmt.Errorf("failed to fetch final standings: %w", err)
	}

	members, err := s.GetLeagueMembers(ctx, league.ID)
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
		addr, err := s.getAccountEVMAddress(ctx, member.WalletAddress)
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
		_, err = s.db.Exec(ctx, `
			UPDATE leagues SET payout_status = 'executed', payouts_executed_at = NOW(), updated_at = NOW() WHERE id = $1
		`, league.ID)
		return err
	}

	leagueIDBytes, err := uuidToBytes32(league.ID)
	if err != nil {
		return fmt.Errorf("invalid league ID: %w", err)
	}
	contractEVM, err := hederaAccountToEVM(s.hederaEscrowContractID)
	if err != nil {
		return fmt.Errorf("invalid escrow contract ID: %w", err)
	}

	recipients := make([][20]byte, len(resolved))
	amounts := make([]int64, len(resolved))
	var totalPayout int64
	for i, r := range resolved {
		recipients[i] = r.evmAddr
		amounts[i] = r.amountCents * 10_000 // cents -> 6-decimal micro-USDC
		totalPayout += amounts[i]
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
		if _, dbErr := s.db.Exec(ctx, `
			UPDATE leagues SET payout_status = 'failed', updated_at = NOW() WHERE id = $1
		`, league.ID); dbErr != nil {
			log.Printf("[oracle] failed to mark league %s as failed: %v", league.ID, dbErr)
		}
		return fmt.Errorf("on-chain execution failed: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		UPDATE leagues
		SET payout_status = 'executed', payout_tx_hash = $2, payouts_executed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, league.ID, txHash); err != nil {
		return fmt.Errorf("failed to update league payout status: %w", err)
	}

	for _, r := range resolved {
		if _, err := s.db.Exec(ctx, `
			UPDATE league_members
			SET final_rank = $1, payout_amount_cents = $2, payout_tx_hash = $3, payout_paid_at = NOW(), updated_at = NOW()
			WHERE league_id = $4 AND roster_id = $5
		`, r.place, r.amountCents, txHash, league.ID, r.member.RosterID); err != nil {
			log.Printf("[oracle] failed to update member %s payout record: %v", r.member.DisplayName, err)
		}
	}

	log.Printf("[oracle] season-end payouts for league %s: %d recipients, tx %s", league.ID, len(resolved), txHash)
	return nil
}
