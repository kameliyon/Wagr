package league

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wagr/src/internal/fantasy"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/sha3"
)

type Service struct {
	db                      *pgxpool.Pool
	platformService         *fantasy.PlatformService
	hederaUSDCTokenID       string
	hederaEscrowContractID  string
	hederaNetwork           string
}

func NewService(db *pgxpool.Pool, platformService *fantasy.PlatformService, hederaUSDCTokenID, hederaEscrowContractID, hederaNetwork string) *Service {
	if hederaNetwork == "" {
		hederaNetwork = "testnet"
	}
	return &Service{
		db:                     db,
		platformService:        platformService,
		hederaUSDCTokenID:      hederaUSDCTokenID,
		hederaEscrowContractID: hederaEscrowContractID,
		hederaNetwork:          hederaNetwork,
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
			team_name, display_name, avatar_url, is_owner, roster_id,
			wins, losses, ties, total_points
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (league_id, platform_user_id)
		DO UPDATE SET
			team_name = EXCLUDED.team_name,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			wins = EXCLUDED.wins,
			losses = EXCLUDED.losses,
			ties = EXCLUDED.ties,
			total_points = EXCLUDED.total_points,
			updated_at = NOW()
		RETURNING id, league_id, user_id, platform, platform_user_id, platform_username,
			team_name, display_name, avatar_url, is_owner, roster_id, wins, losses, ties,
			total_points, COALESCE(wallet_address, ''), payment_status, created_at,
			payment_token, transaction_hash, paid_at
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

		teamName := pm.Metadata["team_name"]

		err = tx.QueryRow(ctx, memberQuery,
			league.ID,
			req.Platform,
			pm.PlatformUserID,
			pm.Username,
			teamName,
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
			&member.TeamName,
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
			&member.PaymentToken,
			&member.TransactionHash,
			&member.PaidAt,
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
			l.cancelled_at, BOOL_OR(lm.is_owner) AS is_commissioner
		FROM leagues l
		JOIN league_members lm ON lm.league_id = l.id
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE pp.user_id = $1
		GROUP BY l.id, l.platform, l.platform_league_id, l.name, l.sport, l.season, l.status,
			l.total_rosters, l.scoring_type, l.entry_fee_cents, l.created_at, l.updated_at,
			l.cancelled_at
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
			&league.CancelledAt,
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
			total_rosters, scoring_type, entry_fee_cents, created_at, updated_at, cancelled_at
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
		&league.CancelledAt,
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
			COALESCE(BOOL_OR(lm.is_owner) FILTER (WHERE pp.user_id IS NOT NULL), false) AS is_commissioner,
			l.cancelled_at
		FROM leagues l
		LEFT JOIN league_members lm ON lm.league_id = l.id
		LEFT JOIN platform_profiles pp ON pp.platform = lm.platform
			AND pp.platform_user_id = lm.platform_user_id
			AND pp.user_id = $2
		WHERE l.id = $1
		GROUP BY l.entry_fee_cents, l.total_rosters, l.payout_structure, l.cancelled_at
	`
	settings := &LeagueSettings{PayoutStructure: []PayoutEntry{}}
	var payoutJSON []byte
	err := s.db.QueryRow(ctx, query, leagueID, userID).Scan(
		&settings.EntryFeeCents,
		&settings.TotalRosters,
		&payoutJSON,
		&settings.IsCommissioner,
		&settings.CancelledAt,
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
		SELECT
			lm.id,
			lm.league_id,
			COALESCE(pp.user_id::text, lm.user_id::text) AS user_id,
			lm.platform,
			lm.platform_user_id,
			lm.platform_username,
			lm.team_name,
			lm.display_name,
			lm.avatar_url,
			lm.is_owner,
			lm.roster_id,
			lm.wins,
			lm.losses,
			lm.ties,
			lm.total_points,
			COALESCE(u.wallet_address, lm.wallet_address, '') AS wallet_address,
			lm.payment_status,
			lm.created_at,
			lm.payment_token,
			lm.transaction_hash,
			lm.paid_at
		FROM league_members lm
		LEFT JOIN platform_profiles pp
			ON pp.platform = lm.platform
			AND pp.platform_user_id = lm.platform_user_id
		LEFT JOIN users u ON u.id = pp.user_id
		WHERE lm.league_id = $1
		ORDER BY lm.roster_id
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
			&member.TeamName,
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
			&member.PaymentToken,
			&member.TransactionHash,
			&member.PaidAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan league member: %w", err)
		}
		members = append(members, member)
	}

	return members, nil
}

// SetPaymentToken sets the payment token (hbar or usdc) for the calling user's membership
func (s *Service) SetPaymentToken(ctx context.Context, leagueID, userID, token string) error {
	if token != "hbar" && token != "usdc" {
		return fmt.Errorf("invalid token: must be 'hbar' or 'usdc'")
	}

	result, err := s.db.Exec(ctx, `
		UPDATE league_members lm
		SET payment_token = $3, updated_at = NOW()
		FROM platform_profiles pp
		WHERE lm.league_id = $1
		  AND lm.platform = pp.platform
		  AND lm.platform_user_id = pp.platform_user_id
		  AND pp.user_id = $2
	`, leagueID, userID, token)
	if err != nil {
		return fmt.Errorf("failed to set payment token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotLeagueMember
	}
	return nil
}

// InitiatePayment returns USDC escrow payment instructions for the calling user's membership
func (s *Service) InitiatePayment(ctx context.Context, leagueID, userID string) (*PaymentInstructions, error) {
	var entryFeeCents int64
	var paymentStatus string

	err := s.db.QueryRow(ctx, `
		SELECT l.entry_fee_cents, lm.payment_status
		FROM league_members lm
		JOIN leagues l ON l.id = lm.league_id
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
	`, leagueID, userID).Scan(&entryFeeCents, &paymentStatus)

	if err == pgx.ErrNoRows {
		return nil, ErrNotLeagueMember
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch member payment info: %w", err)
	}

	if paymentStatus == "paid" {
		return nil, ErrAlreadyPaid
	}

	// entry_fee_cents → 6-decimal USDC: $50.00 = 5000 cents = 50_000_000 micro-USDC
	amountUSDC := entryFeeCents * 10_000
	dollars := float64(entryFeeCents) / 100.0

	return &PaymentInstructions{
		ContractID:      s.hederaEscrowContractID,
		USDCTokenID:     s.hederaUSDCTokenID,
		AmountUSDC:      amountUSDC,
		AmountFormatted: fmt.Sprintf("$%.2f USDC", dollars),
	}, nil
}

// ConfirmPayment verifies the on-chain payment via Mirror Node and marks the member as paid
func (s *Service) ConfirmPayment(ctx context.Context, leagueID, userID, transactionID string) error {
	var walletAddress string
	var entryFeeCents int64
	var paymentStatus string

	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(u.wallet_address, lm.wallet_address, ''), l.entry_fee_cents, lm.payment_status
		FROM league_members lm
		JOIN leagues l ON l.id = lm.league_id
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		LEFT JOIN users u ON u.id = pp.user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
	`, leagueID, userID).Scan(&walletAddress, &entryFeeCents, &paymentStatus)

	if err == pgx.ErrNoRows {
		return ErrNotLeagueMember
	}
	if err != nil {
		return fmt.Errorf("failed to fetch member info: %w", err)
	}

	if paymentStatus == "paid" {
		return ErrAlreadyPaid
	}

	evmAddr, err := s.getAccountEVMAddress(ctx, walletAddress)
	if err != nil {
		return fmt.Errorf("invalid wallet address %q: %w", walletAddress, err)
	}

	leagueIdBytes, err := uuidToBytes32(leagueID)
	if err != nil {
		return fmt.Errorf("invalid league ID: %w", err)
	}

	contractEVM, err := hederaAccountToEVM(s.hederaEscrowContractID)
	if err != nil {
		return fmt.Errorf("invalid contract ID: %w", err)
	}

	callData := encodePaymentsCall(leagueIdBytes, evmAddr)

	paid, err := s.readContractPayment(ctx, contractEVM, callData)
	if err != nil {
		return fmt.Errorf("failed to verify on-chain payment: %w", err)
	}

	// entry_fee_cents * 10_000 = required 6-decimal USDC amount
	requiredUSDC := entryFeeCents * 10_000
	if paid < requiredUSDC {
		return ErrPaymentInsufficient
	}

	_, err = s.db.Exec(ctx, `
		UPDATE league_members lm
		SET payment_status = 'paid', transaction_hash = $3, paid_at = NOW(), updated_at = NOW()
		FROM platform_profiles pp
		WHERE lm.league_id = $1
		  AND lm.platform = pp.platform
		  AND lm.platform_user_id = pp.platform_user_id
		  AND pp.user_id = $2
	`, leagueID, userID, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	return nil
}

// getAccountEVMAddress fetches the canonical EVM address for a Hedera account from the Mirror Node.
// ECDSA (HashPack) accounts have a key-derived alias that differs from the long-zero format;
// using the Mirror Node ensures msg.sender in contracts matches the lookup address.
func (s *Service) getAccountEVMAddress(ctx context.Context, accountID string) ([20]byte, error) {
	mirrorURL := fmt.Sprintf("https://%s.mirrornode.hedera.com/api/v1/accounts/%s", s.hederaNetwork, accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mirrorURL, nil)
	if err != nil {
		return hederaAccountToEVM(accountID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return hederaAccountToEVM(accountID)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return hederaAccountToEVM(accountID)
	}
	var result struct {
		EvmAddress string `json:"evm_address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.EvmAddress == "" {
		return hederaAccountToEVM(accountID)
	}
	evmHex := strings.TrimPrefix(result.EvmAddress, "0x")
	if len(evmHex) != 40 {
		return hederaAccountToEVM(accountID)
	}
	addrBytes, err := hex.DecodeString(evmHex)
	if err != nil {
		return hederaAccountToEVM(accountID)
	}
	var addr [20]byte
	copy(addr[:], addrBytes)
	return addr, nil
}

// hederaAccountToEVM converts "0.0.NNNNN" to a 20-byte EVM address (shard.realm ignored, num in last 8 bytes)
func hederaAccountToEVM(accountID string) ([20]byte, error) {
	parts := strings.Split(accountID, ".")
	if len(parts) != 3 {
		return [20]byte{}, fmt.Errorf("expected format 0.0.NNNNN, got %q", accountID)
	}
	var num uint64
	_, err := fmt.Sscanf(parts[2], "%d", &num)
	if err != nil {
		return [20]byte{}, fmt.Errorf("invalid account number %q: %w", parts[2], err)
	}
	var addr [20]byte
	binary.BigEndian.PutUint64(addr[12:], num)
	return addr, nil
}

// uuidToBytes32 converts a UUID string to a 32-byte array (UUID bytes left-aligned, right-padded with zeros)
func uuidToBytes32(uuidStr string) ([32]byte, error) {
	hexStr := strings.ReplaceAll(uuidStr, "-", "")
	if len(hexStr) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32-char UUID hex, got %d chars from %q", len(hexStr), uuidStr)
	}
	uuidBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to decode UUID hex: %w", err)
	}
	var result [32]byte
	copy(result[:], uuidBytes) // 16 bytes left-aligned, remaining 16 bytes are zero
	return result, nil
}

// encodePaymentsCall ABI-encodes a call to payments(bytes32,address)
func encodePaymentsCall(leagueId [32]byte, member [20]byte) []byte {
	// Function selector: keccak256("payments(bytes32,address)")[0:4]
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("payments(bytes32,address)"))
	selector := h.Sum(nil)[:4]

	// ABI encode: bytes32 (32 bytes as-is) + address (20 bytes, left-padded to 32)
	var addrPadded [32]byte
	copy(addrPadded[12:], member[:])

	callData := make([]byte, 0, 68)
	callData = append(callData, selector...)
	callData = append(callData, leagueId[:]...)
	callData = append(callData, addrPadded[:]...)
	return callData
}

// readContractPayment calls the Mirror Node to read payments(bytes32,address) from the escrow contract
func (s *Service) readContractPayment(ctx context.Context, contractAddr [20]byte, callData []byte) (int64, error) {
	mirrorNodeURL := fmt.Sprintf("https://%s.mirrornode.hedera.com", s.hederaNetwork)

	type contractCallReq struct {
		Block    string `json:"block"`
		Data     string `json:"data"`
		To       string `json:"to"`
		Estimate bool   `json:"estimate"`
	}

	reqBody := contractCallReq{
		Block:    "latest",
		Data:     "0x" + hex.EncodeToString(callData),
		To:       "0x" + hex.EncodeToString(contractAddr[:]),
		Estimate: false,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		mirrorNodeURL+"/api/v1/contracts/call",
		bytes.NewReader(reqJSON),
	)
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("mirror node request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("mirror node returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode mirror node response: %w", err)
	}

	// Result is ABI-encoded uint256: "0x" + 64 hex chars (32 bytes big-endian)
	hexResult := strings.TrimPrefix(result.Result, "0x")
	if len(hexResult) < 64 {
		return 0, fmt.Errorf("unexpected result length from mirror node: %q", result.Result)
	}
	// Take last 64 hex chars (32 bytes)
	hexResult = hexResult[len(hexResult)-64:]

	resultBytes, err := hex.DecodeString(hexResult)
	if err != nil {
		return 0, fmt.Errorf("failed to decode result hex: %w", err)
	}

	// Parse last 8 bytes as uint64 (amounts fit well within int64 for any reasonable entry fee)
	amount := int64(binary.BigEndian.Uint64(resultBytes[24:]))
	return amount, nil
}

// CancelLeague marks a league as cancelled; only the commissioner may do this
func (s *Service) CancelLeague(ctx context.Context, leagueID, userID string) error {
	var isCommissioner bool
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(BOOL_OR(lm.is_owner), false)
		FROM league_members lm
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
		GROUP BY lm.league_id
	`, leagueID, userID).Scan(&isCommissioner)
	if err != nil || !isCommissioner {
		return ErrNotCommissioner
	}

	result, err := s.db.Exec(ctx, `
		UPDATE leagues SET cancelled_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND cancelled_at IS NULL
	`, leagueID)
	if err != nil {
		return fmt.Errorf("failed to cancel league: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrLeagueAlreadyCancelled
	}
	return nil
}

// ReactivateLeague clears cancelled_at and resets refunded members to unpaid; commissioner only
func (s *Service) ReactivateLeague(ctx context.Context, leagueID, userID string) error {
	var isCommissioner bool
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(BOOL_OR(lm.is_owner), false)
		FROM league_members lm
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
		GROUP BY lm.league_id
	`, leagueID, userID).Scan(&isCommissioner)
	if err != nil || !isCommissioner {
		return ErrNotCommissioner
	}

	result, err := s.db.Exec(ctx, `
		UPDATE leagues SET cancelled_at = NULL, updated_at = NOW()
		WHERE id = $1 AND cancelled_at IS NOT NULL
	`, leagueID)
	if err != nil {
		return fmt.Errorf("failed to reactivate league: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrLeagueNotCancelled
	}

	_, err = s.db.Exec(ctx, `
		UPDATE league_members SET payment_status = 'unpaid', transaction_hash = NULL, updated_at = NOW()
		WHERE league_id = $1 AND payment_status = 'refunded'
	`, leagueID)
	if err != nil {
		return fmt.Errorf("failed to reset refunded members: %w", err)
	}
	return nil
}

// ConfirmRefund verifies the on-chain refund via Mirror Node and marks the member as refunded
func (s *Service) ConfirmRefund(ctx context.Context, leagueID, userID, transactionID string) error {
	var walletAddress string
	var paymentStatus string
	var cancelledAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(u.wallet_address, lm.wallet_address, ''), lm.payment_status, l.cancelled_at
		FROM league_members lm
		JOIN leagues l ON l.id = lm.league_id
		JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
		LEFT JOIN users u ON u.id = pp.user_id
		WHERE lm.league_id = $1 AND pp.user_id = $2
	`, leagueID, userID).Scan(&walletAddress, &paymentStatus, &cancelledAt)

	if err == pgx.ErrNoRows {
		return ErrNotLeagueMember
	}
	if err != nil {
		return fmt.Errorf("failed to fetch member info: %w", err)
	}
	if paymentStatus != "paid" {
		return ErrPaymentInsufficient
	}
	if cancelledAt == nil {
		return ErrLeagueNotCancelled
	}

	evmAddr, err := s.getAccountEVMAddress(ctx, walletAddress)
	if err != nil {
		return fmt.Errorf("invalid wallet address %q: %w", walletAddress, err)
	}

	leagueIdBytes, err := uuidToBytes32(leagueID)
	if err != nil {
		return fmt.Errorf("invalid league ID: %w", err)
	}

	contractEVM, err := hederaAccountToEVM(s.hederaEscrowContractID)
	if err != nil {
		return fmt.Errorf("invalid contract ID: %w", err)
	}

	callData := encodePaymentsCall(leagueIdBytes, evmAddr)

	onChain, err := s.readContractPayment(ctx, contractEVM, callData)
	if err != nil {
		return fmt.Errorf("failed to verify on-chain refund: %w", err)
	}
	if onChain != 0 {
		return ErrPaymentInsufficient
	}

	_, err = s.db.Exec(ctx, `
		UPDATE league_members lm
		SET payment_status = 'refunded', transaction_hash = $3, updated_at = NOW()
		FROM platform_profiles pp
		WHERE lm.league_id = $1
		  AND lm.platform = pp.platform
		  AND lm.platform_user_id = pp.platform_user_id
		  AND pp.user_id = $2
	`, leagueID, userID, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update refund status: %w", err)
	}
	return nil
}

// GetPaymentStatus returns the full member list for a league, gated to league members only
func (s *Service) GetPaymentStatus(ctx context.Context, leagueID, userID string) ([]LeagueMember, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM league_members lm
			JOIN platform_profiles pp ON pp.platform = lm.platform AND pp.platform_user_id = lm.platform_user_id
			WHERE lm.league_id = $1 AND pp.user_id = $2
		)
	`, leagueID, userID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to verify league membership: %w", err)
	}
	if !exists {
		return nil, ErrNotLeagueMember
	}

	return s.GetLeagueMembers(ctx, leagueID)
}

