package league

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"wagr/src/internal/fantasy"
	"wagr/src/internal/fantasy/sleeper"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db              *pgxpool.Pool
	platformService *fantasy.PlatformService
	sleeperClient   *sleeper.Client // DEPRECATED: Kept for backward compatibility
}

func NewService(db *pgxpool.Pool, platformService *fantasy.PlatformService) *Service {
	return &Service{
		db:              db,
		platformService: platformService,
	}
}

// NewServiceWithSleeperClient creates a service with legacy Sleeper client support
// DEPRECATED: Use NewService with PlatformService instead
func NewServiceWithSleeperClient(db *pgxpool.Pool, sleeperClient *sleeper.Client) *Service {
	return &Service{
		db:            db,
		sleeperClient: sleeperClient,
	}
}

// LinkPlatformProfile links a user's WAGR account to their fantasy platform account
func (s *Service) LinkPlatformProfile(ctx context.Context, userID string, req LinkPlatformRequest) (*PlatformProfile, error) {
	// Fetch user from platform
	platformUser, err := s.platformService.GetUser(ctx, fantasy.PlatformType(req.Platform), req.PlatformUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user from %s: %w", req.Platform, err)
	}

	// Insert or update platform profile
	query := `
		INSERT INTO platform_profiles (user_id, platform, platform_user_id, platform_username, display_name, avatar_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, platform, platform_user_id)
		DO UPDATE SET
			platform_username = EXCLUDED.platform_username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id, user_id, platform, platform_user_id, platform_username, display_name, avatar_url, created_at, updated_at
	`

	var profile PlatformProfile
	err = s.db.QueryRow(ctx, query,
		userID,
		req.Platform,
		platformUser.PlatformUserID,
		platformUser.Username,
		platformUser.DisplayName,
		platformUser.AvatarURL,
	).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Platform,
		&profile.PlatformUserID,
		&profile.PlatformUsername,
		&profile.DisplayName,
		&profile.AvatarURL,
		&profile.LinkedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to link platform profile: %w", err)
	}

	return &profile, nil
}

// ImportLeague imports a fantasy league from a platform into WAGR
func (s *Service) ImportLeague(ctx context.Context, userID string, req ImportLeagueRequest) (*ImportLeagueResponse, error) {
	platformType := fantasy.PlatformType(req.Platform)

	// Fetch league details from platform
	platformLeague, err := s.platformService.GetLeague(ctx, platformType, req.PlatformLeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league from %s: %w", req.Platform, err)
	}

	// Fetch league members
	platformMembers, err := s.platformService.GetLeagueMembers(ctx, platformType, req.PlatformLeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league members: %w", err)
	}

	// Fetch rosters for stats
	platformRosters, err := s.platformService.GetLeagueRosters(ctx, platformType, req.PlatformLeagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league rosters: %w", err)
	}

	// Create roster map for quick lookup
	rosterMap := make(map[string]*fantasy.PlatformRoster)
	for i := range platformRosters {
		rosterMap[platformRosters[i].OwnerID] = &platformRosters[i]
	}

	// Begin transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert or update league
	leagueQuery := `
		INSERT INTO leagues (platform, platform_league_id, name, sport, season, status, total_rosters, scoring_type, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (platform, platform_league_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			total_rosters = EXCLUDED.total_rosters,
			updated_at = NOW()
		RETURNING id, platform, platform_league_id, name, sport, season, status, total_rosters, scoring_type, entry_fee_cents, created_at, updated_at
	`

	var league League
	err = tx.QueryRow(ctx, leagueQuery,
		req.Platform,
		platformLeague.PlatformLeagueID,
		platformLeague.Name,
		platformLeague.Sport,
		platformLeague.Season,
		platformLeague.Status,
		platformLeague.TotalRosters,
		platformLeague.ScoringType,
		platformLeague.Metadata,
	).Scan(
		&league.ID,
		&league.Platform,
		&league.PlatformLeagueID,
		&league.Name,
		&league.Sport,
		&league.Season,
		&league.Status,
		&league.TotalRosters,
		&league.ScoringType,
		&league.EntryFeeCents,
		&league.ImportedAt,
		&league.LastSyncedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to insert league: %w", err)
	}

	// Insert or update league members
	memberQuery := `
		INSERT INTO league_members (
			league_id, platform, platform_user_id, platform_username,
			display_name, avatar_url, is_owner, roster_id,
			wins, losses, ties, total_points
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (league_id, platform_user_id)
		DO UPDATE SET
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			wins = EXCLUDED.wins,
			losses = EXCLUDED.losses,
			ties = EXCLUDED.ties,
			total_points = EXCLUDED.total_points,
			updated_at = NOW()
		RETURNING id, league_id, user_id, platform, platform_user_id, platform_username,
			display_name, avatar_url, is_owner, roster_id, wins, losses, ties,
			total_points, COALESCE(wallet_address, ''), payment_status, created_at
	`

	members := make([]LeagueMember, 0, len(platformMembers))
	for _, pm := range platformMembers {
		var member LeagueMember

		// Get roster stats
		roster := rosterMap[pm.PlatformUserID]
		wins, losses, ties, totalPoints := 0, 0, 0, 0.0
		if roster != nil {
			wins = roster.Wins
			losses = roster.Losses
			ties = roster.Ties
			totalPoints = roster.TotalPoints
		}

		err = tx.QueryRow(ctx, memberQuery,
			league.ID,
			req.Platform,
			pm.PlatformUserID,
			pm.Username,
			pm.DisplayName,
			pm.AvatarURL,
			pm.IsOwner,
			pm.RosterID,
			wins,
			losses,
			ties,
			totalPoints,
		).Scan(
			&member.ID,
			&member.LeagueID,
			&member.UserID,
			&member.Platform,
			&member.PlatformUserID,
			&member.PlatformUsername,
			&member.DisplayName,
			&member.AvatarURL,
			&member.IsOwner,
			&member.RosterID,
			&member.Wins,
			&member.Losses,
			&member.Ties,
			&member.TotalPoints,
			&member.WalletAddress,
			&member.PaymentStatus,
			&member.JoinedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to insert league member: %w", err)
		}

		members = append(members, member)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	league.ImportedBy = userID
	league.ImportedAt = time.Now()

	return &ImportLeagueResponse{
		League:  league,
		Members: members,
	}, nil
}

// GetUserLeagues returns all leagues where the user is a member (matched via platform_profiles)
func (s *Service) GetUserLeagues(ctx context.Context, userID string) ([]League, error) {
	query := `
		SELECT l.id, l.platform, l.platform_league_id, l.name, l.sport, l.season, l.status,
			l.total_rosters, l.scoring_type, l.entry_fee_cents, l.created_at, l.updated_at,
			BOOL_OR(lm.is_owner) AS is_commissioner
		FROM leagues l
		JOIN league_members lm ON lm.league_id = l.id
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE pp.user_id = $1
		GROUP BY l.id, l.platform, l.platform_league_id, l.name, l.sport, l.season, l.status,
			l.total_rosters, l.scoring_type, l.entry_fee_cents, l.created_at, l.updated_at
		ORDER BY l.created_at DESC
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user leagues: %w", err)
	}
	defer rows.Close()

	var leagues []League
	for rows.Next() {
		var league League
		err := rows.Scan(
			&league.ID,
			&league.Platform,
			&league.PlatformLeagueID,
			&league.Name,
			&league.Sport,
			&league.Season,
			&league.Status,
			&league.TotalRosters,
			&league.ScoringType,
			&league.EntryFeeCents,
			&league.ImportedAt,
			&league.LastSyncedAt,
			&league.IsCommissioner,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan league: %w", err)
		}
		leagues = append(leagues, league)
	}

	return leagues, nil
}

// GetLeague returns a league by ID
func (s *Service) GetLeague(ctx context.Context, leagueID string) (*League, error) {
	query := `
		SELECT id, platform, platform_league_id, name, sport, season, status,
			total_rosters, scoring_type, entry_fee_cents, created_at, updated_at
		FROM leagues
		WHERE id = $1
	`

	var league League
	err := s.db.QueryRow(ctx, query, leagueID).Scan(
		&league.ID,
		&league.Platform,
		&league.PlatformLeagueID,
		&league.Name,
		&league.Sport,
		&league.Season,
		&league.Status,
		&league.TotalRosters,
		&league.ScoringType,
		&league.EntryFeeCents,
		&league.ImportedAt,
		&league.LastSyncedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get league: %w", err)
	}

	return &league, nil
}

// DeleteLeague removes the calling user's membership from a league.
// If no WAGR-linked members remain afterward, the league record itself is also deleted.
func (s *Service) DeleteLeague(ctx context.Context, leagueID, userID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM league_members
		USING platform_profiles
		WHERE league_members.league_id = $1
		  AND league_members.platform = platform_profiles.platform
		  AND league_members.platform_user_id = platform_profiles.platform_user_id
		  AND platform_profiles.user_id = $2
	`, leagueID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove league membership: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("league not found or user is not a member")
	}

	// Clean up the league record if no WAGR users are members anymore
	var remaining int
	err = s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM league_members lm
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE lm.league_id = $1
	`, leagueID).Scan(&remaining)
	if err != nil {
		return fmt.Errorf("failed to check remaining members: %w", err)
	}
	if remaining == 0 {
		_, err = s.db.Exec(ctx, `DELETE FROM leagues WHERE id = $1`, leagueID)
		if err != nil {
			return fmt.Errorf("failed to delete league: %w", err)
		}
	}
	return nil
}

// GetLeagueSettings returns entry fee, payout structure, and commissioner status for a league
func (s *Service) GetLeagueSettings(ctx context.Context, leagueID, userID string) (*LeagueSettings, error) {
	query := `
		SELECT l.entry_fee_cents, l.total_rosters, l.payout_structure,
			COALESCE(BOOL_OR(lm.is_owner), false) AS is_commissioner
		FROM leagues l
		LEFT JOIN league_members lm ON lm.league_id = l.id
		LEFT JOIN platform_profiles pp ON pp.platform = lm.platform
			AND pp.platform_user_id = lm.platform_user_id
			AND pp.user_id = $2
		WHERE l.id = $1
		GROUP BY l.entry_fee_cents, l.total_rosters, l.payout_structure
	`
	settings := &LeagueSettings{PayoutStructure: []PayoutEntry{}}
	var payoutJSON []byte
	err := s.db.QueryRow(ctx, query, leagueID, userID).Scan(
		&settings.EntryFeeCents,
		&settings.TotalRosters,
		&payoutJSON,
		&settings.IsCommissioner,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get league settings: %w", err)
	}
	if payoutJSON != nil {
		json.Unmarshal(payoutJSON, &settings.PayoutStructure)
	}
	return settings, nil
}

// UpdateLeagueSettings updates entry fee and payout structure; only the commissioner may do this
func (s *Service) UpdateLeagueSettings(ctx context.Context, leagueID, userID string, req UpdateSettingsRequest) (*LeagueSettings, error) {
	var isCommissioner bool
	checkQuery := `
		SELECT COALESCE(BOOL_OR(lm.is_owner), false)
		FROM league_members lm
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
		GROUP BY lm.league_id
	`
	err := s.db.QueryRow(ctx, checkQuery, leagueID, userID).Scan(&isCommissioner)
	if err != nil || !isCommissioner {
		return nil, ErrNotCommissioner
	}

	payoutJSON, err := json.Marshal(req.PayoutStructure)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payout structure: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE leagues
		SET entry_fee_cents = $2, payout_structure = $3::jsonb, updated_at = NOW()
		WHERE id = $1
	`, leagueID, req.EntryFeeCents, payoutJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update league settings: %w", err)
	}

	return s.GetLeagueSettings(ctx, leagueID, userID)
}

// GetLeagueMembers returns all members of a league
func (s *Service) GetLeagueMembers(ctx context.Context, leagueID string) ([]LeagueMember, error) {
	query := `
		SELECT id, league_id, user_id, platform, platform_user_id, platform_username,
			display_name, avatar_url, is_owner, roster_id, wins, losses, ties,
			total_points, COALESCE(wallet_address, ''), payment_status, created_at
		FROM league_members
		WHERE league_id = $1
		ORDER BY roster_id
	`

	rows, err := s.db.Query(ctx, query, leagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to query league members: %w", err)
	}
	defer rows.Close()

	var members []LeagueMember
	for rows.Next() {
		var member LeagueMember
		err := rows.Scan(
			&member.ID,
			&member.LeagueID,
			&member.UserID,
			&member.Platform,
			&member.PlatformUserID,
			&member.PlatformUsername,
			&member.DisplayName,
			&member.AvatarURL,
			&member.IsOwner,
			&member.RosterID,
			&member.Wins,
			&member.Losses,
			&member.Ties,
			&member.TotalPoints,
			&member.WalletAddress,
			&member.PaymentStatus,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan league member: %w", err)
		}
		members = append(members, member)
	}

	return members, nil
}


