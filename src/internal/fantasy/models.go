package fantasy

import (
	"time"
)

// PlatformUser represents a user on any fantasy sports platform
type PlatformUser struct {
	PlatformUserID string            `json:"platform_user_id"` // Platform-specific user ID
	Username       string            `json:"username"`
	DisplayName    string            `json:"display_name"`
	AvatarURL      string            `json:"avatar_url,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"` // Platform-specific fields
}

// PlatformLeague represents a fantasy league on any platform
type PlatformLeague struct {
	PlatformLeagueID string            `json:"platform_league_id"` // Platform-specific league ID
	Name             string            `json:"name"`
	Sport            string            `json:"sport"`        // e.g., "nfl", "nba", "mlb"
	Season           string            `json:"season"`       // e.g., "2024"
	Status           string            `json:"status"`       // e.g., "pre_draft", "in_season", "complete"
	TotalRosters     int               `json:"total_rosters"`
	ScoringType      string            `json:"scoring_type"` // e.g., "ppr", "standard", "half_ppr"
	Metadata         map[string]string `json:"metadata,omitempty"` // Platform-specific fields
}

// PlatformMember represents a league member with their profile information
type PlatformMember struct {
	PlatformUserID string            `json:"platform_user_id"`
	Username       string            `json:"username"`
	DisplayName    string            `json:"display_name"`
	AvatarURL      string            `json:"avatar_url,omitempty"`
	IsOwner        bool              `json:"is_owner"`   // League commissioner/owner
	RosterID       int               `json:"roster_id"`  // Their roster number in the league
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// PlatformRoster represents a team roster in a league
type PlatformRoster struct {
	RosterID       int               `json:"roster_id"`
	OwnerID        string            `json:"owner_id"` // Platform-specific user ID
	Wins           int               `json:"wins"`
	Losses         int               `json:"losses"`
	Ties           int               `json:"ties"`
	TotalPoints    float64           `json:"total_points"`
	Players        []string          `json:"players,omitempty"` // Platform-specific player IDs
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// PlatformMatchup represents a single team's result in a weekly matchup
type PlatformMatchup struct {
	RosterID  int     `json:"roster_id"`
	MatchupID int     `json:"matchup_id"` // teams sharing the same matchup_id played each other
	Points    float64 `json:"points"`
	Week      int     `json:"week"`
}

// PlatformStanding represents a team's final placement in the playoffs (winners bracket only)
type PlatformStanding struct {
	RosterID int `json:"roster_id"`
	Place    int `json:"place"` // 1 = champion, 2 = runner-up, etc.
}

// PlatformCredentials stores authentication credentials for platforms that require auth
type PlatformCredentials struct {
	Platform     PlatformType `json:"platform"`
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	APIKey       string       `json:"api_key,omitempty"` // For platforms using API keys
	Secret       string       `json:"secret,omitempty"`  // For OAuth secrets
}
