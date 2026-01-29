package league

import "time"

// SleeperProfile represents a link between a WAGR user and their Sleeper account
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

// League represents an imported Sleeper fantasy league
type League struct {
	ID              int        `json:"id"`
	SleeperLeagueID string     `json:"sleeper_league_id"`
	Name            string     `json:"name"`
	Season          string     `json:"season"`
	Sport           string     `json:"sport"`
	Status          string     `json:"status"`
	TotalRosters    int        `json:"total_rosters"`
	ImportedBy      string     `json:"imported_by"`
	ImportedAt      time.Time  `json:"imported_at"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
}

// LeagueMember represents a member of a league with their roster information
type LeagueMember struct {
	ID            int       `json:"id"`
	LeagueID      int       `json:"league_id"`
	UserID        *string   `json:"user_id,omitempty"`
	SleeperUserID string    `json:"sleeper_user_id"`
	RosterID      int       `json:"roster_id"`
	DisplayName   string    `json:"display_name"`
	IsOwner       bool      `json:"is_owner"`
	JoinedAt      time.Time `json:"joined_at"`
}

// LinkSleeperRequest is the request payload for linking a Sleeper account
type LinkSleeperRequest struct {
	SleeperUsername string `json:"sleeper_username"`
}

// ImportLeagueRequest is the request payload for importing a league
type ImportLeagueRequest struct {
	SleeperLeagueID string `json:"sleeper_league_id"`
}

// ImportLeagueResponse is the response after importing a league
type ImportLeagueResponse struct {
	League  League         `json:"league"`
	Members []LeagueMember `json:"members"`
}
