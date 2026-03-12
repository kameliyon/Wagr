package league

import (
	"errors"
	"time"
)

// PlatformProfile represents a link between a WAGR user and their fantasy platform account
type PlatformProfile struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Platform         string    `json:"platform"`          // 'sleeper', 'espn', 'yahoo'
	PlatformUserID   string    `json:"platform_user_id"`
	PlatformUsername string    `json:"platform_username"`
	DisplayName      string    `json:"display_name,omitempty"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	LinkedAt         time.Time `json:"linked_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// League represents an imported fantasy league from any platform
type League struct {
	ID               string     `json:"id"`
	Platform         string     `json:"platform"`           // 'sleeper', 'espn', 'yahoo'
	PlatformLeagueID string     `json:"platform_league_id"`
	Name             string     `json:"name"`
	Season           string     `json:"season"`
	Sport            string     `json:"sport"`
	Status           string     `json:"status"`
	TotalRosters     int        `json:"total_rosters"`
	ScoringType      string     `json:"scoring_type,omitempty"`
	EntryFeeCents    int64      `json:"entry_fee_cents"`
	IsCommissioner   bool       `json:"is_commissioner"`
	ImportedBy       string     `json:"imported_by"`
	ImportedAt       time.Time  `json:"imported_at"`
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
}

// LeagueMember represents a member of a league with their roster information
type LeagueMember struct {
	ID               string    `json:"id"`
	LeagueID         string    `json:"league_id"`
	UserID           *string   `json:"user_id,omitempty"` // WAGR user ID (null if not a WAGR user)
	Platform         string    `json:"platform"`
	PlatformUserID   string    `json:"platform_user_id"`
	PlatformUsername string    `json:"platform_username,omitempty"`
	TeamName         string    `json:"team_name,omitempty"`
	DisplayName      string    `json:"display_name"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	IsOwner          bool      `json:"is_owner"`
	RosterID         int       `json:"roster_id"`
	Wins             int       `json:"wins"`
	Losses           int       `json:"losses"`
	Ties             int       `json:"ties"`
	TotalPoints      float64   `json:"total_points"`
	WalletAddress    string     `json:"wallet_address,omitempty"`
	PaymentStatus    string     `json:"payment_status"` // 'unpaid', 'paid', 'refunded'
	PaymentToken     *string    `json:"payment_token,omitempty"`    // 'hbar' | 'usdc' | null
	TransactionHash  *string    `json:"transaction_hash,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	JoinedAt         time.Time  `json:"joined_at"`
}

// LinkPlatformRequest is the request payload for linking a fantasy platform account
type LinkPlatformRequest struct {
	Platform         string `json:"platform"`          // 'sleeper', 'espn', 'yahoo'
	PlatformUsername string `json:"platform_username"` // Username to lookup on the platform
}

// ImportLeagueRequest is the request payload for importing a league
type ImportLeagueRequest struct {
	Platform         string `json:"platform"`           // 'sleeper', 'espn', 'yahoo'
	PlatformLeagueID string `json:"platform_league_id"` // League ID on the platform
}

// ImportLeagueResponse is the response after importing a league
type ImportLeagueResponse struct {
	League  League         `json:"league"`
	Members []LeagueMember `json:"members"`
}

var ErrNotCommissioner = errors.New("user is not commissioner of this league")
var ErrNotLeagueMember = errors.New("user is not a member of this league")

// SetPaymentTokenRequest is the request payload for selecting a payment token
type SetPaymentTokenRequest struct {
	Token string `json:"token"` // "hbar" or "usdc"
}

// PayStubResponse is the response for initiating a payment
type PayStubResponse struct {
	Status          string `json:"status"`                     // "pending_signature"
	Token           string `json:"token"`
	AmountCents     int64  `json:"amount_cents"`
	AmountFormatted string `json:"amount_formatted"`           // e.g. "50.00 USDC" or "~X HBAR"
	RecipientNote   string `json:"recipient_note"`
	USDCTokenID     string `json:"usdc_token_id,omitempty"`
	Message         string `json:"message"`
}

// BonusCriteria holds type-specific parameters for weekly bonus entries.
type BonusCriteria struct {
	Threshold *float64 `json:"threshold,omitempty"` // points required (score_threshold)
}

type PayoutEntry struct {
	Type        string         `json:"type"`                 // "placement" or "weekly"
	BonusType   string         `json:"bonus_type,omitempty"` // "weekly_high_score" | "score_threshold"
	Label       string         `json:"label"`
	Place       int            `json:"place,omitempty"` // placement entries only
	AmountCents int64          `json:"amount_cents"`
	Weeks       int            `json:"weeks,omitempty"` // weekly entries: number of occurrences
	Criteria    *BonusCriteria `json:"criteria,omitempty"`
}

type LeagueSettings struct {
	EntryFeeCents   int64         `json:"entry_fee_cents"`
	TotalRosters    int           `json:"total_rosters"`
	PayoutStructure []PayoutEntry `json:"payout_structure"`
	IsCommissioner  bool          `json:"is_commissioner"`
}

type UpdateSettingsRequest struct {
	EntryFeeCents   int64         `json:"entry_fee_cents"`
	PayoutStructure []PayoutEntry `json:"payout_structure"`
}
