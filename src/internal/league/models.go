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

// SleeperProfile represents a link between a WAGR user and their Sleeper account
// DEPRECATED: Use PlatformProfile instead. Kept for backward compatibility.
type SleeperProfile struct {
	ID                 int       `json:"id"`
	UserID             string    `json:"user_id"`
	SleeperUserID      string    `json:"sleeper_user_id"`
	SleeperUsername    string    `json:"sleeper_username"`
	SleeperDisplayName string    `json:"sleeper_display_name,omitempty"`
	SleeperAvatar      string    `json:"sleeper_avatar,omitempty"`
	LinkedAt           time.Time `json:"linked_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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
	DisplayName      string    `json:"display_name"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	IsOwner          bool      `json:"is_owner"`
	RosterID         int       `json:"roster_id"`
	Wins             int       `json:"wins"`
	Losses           int       `json:"losses"`
	Ties             int       `json:"ties"`
	TotalPoints      float64   `json:"total_points"`
	WalletAddress    string    `json:"wallet_address,omitempty"`
	PaymentStatus    string    `json:"payment_status"` // 'unpaid', 'paid', 'refunded'
	JoinedAt         time.Time `json:"joined_at"`
}

// LinkPlatformRequest is the request payload for linking a fantasy platform account
type LinkPlatformRequest struct {
	Platform         string `json:"platform"`          // 'sleeper', 'espn', 'yahoo'
	PlatformUsername string `json:"platform_username"` // Username to lookup on the platform
}

// LinkSleeperRequest is the request payload for linking a Sleeper account
// DEPRECATED: Use LinkPlatformRequest instead. Kept for backward compatibility.
type LinkSleeperRequest struct {
	SleeperUsername string `json:"sleeper_username"`
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

type PayoutEntry struct {
	Type        string `json:"type"`            // "placement" or "weekly"
	Label       string `json:"label"`
	Place       int    `json:"place,omitempty"` // placement entries only
	AmountCents int64  `json:"amount_cents"`
	Weeks       int    `json:"weeks,omitempty"` // weekly entries: number of occurrences
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
