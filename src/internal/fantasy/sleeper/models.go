package sleeper

// User represents a Sleeper user account
type User struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
}

// League represents a Sleeper fantasy league
type League struct {
	LeagueID     string         `json:"league_id"`
	Name         string         `json:"name"`
	Season       string         `json:"season"`
	Sport        string         `json:"sport"`
	TotalRosters int            `json:"total_rosters"`
	Status       string         `json:"status"`
	Settings     LeagueSettings `json:"settings"`
}

// LeagueSettings contains league configuration
type LeagueSettings struct {
	PlayoffWeekStart int `json:"playoff_week_start"`
	LeagueType       int `json:"type"` // 0 = redraft, 1 = keeper, 2 = dynasty
}

// LeagueUser represents a user within a league
type LeagueUser struct {
	UserID      string            `json:"user_id"`
	DisplayName string            `json:"display_name"`
	Avatar      string            `json:"avatar"`
	IsOwner     bool              `json:"is_owner"` // true if this user is the league commissioner
	Metadata    map[string]string `json:"metadata"`
}

// Roster represents a team's roster in a league
type Roster struct {
	RosterID int            `json:"roster_id"`
	OwnerID  string         `json:"owner_id"`
	LeagueID string         `json:"league_id"`
	Players  []string       `json:"players"`
	Starters []string       `json:"starters"`
	Reserve  []string       `json:"reserve"`
	Settings RosterSettings `json:"settings"`
}

// RosterSettings contains roster record info
type RosterSettings struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	Ties   int `json:"ties"`
	Fpts   int `json:"fpts"`
}

// Team combines roster and owner information for API responses
type Team struct {
	RosterID    int      `json:"roster_id"`
	OwnerID     string   `json:"owner_id"`
	DisplayName string   `json:"display_name"`
	Avatar      string   `json:"avatar"`
	Players     []string `json:"players"`
	Wins        int      `json:"wins"`
	Losses      int      `json:"losses"`
	Ties        int      `json:"ties"`
}

// Matchup represents a weekly matchup result for a single team
type Matchup struct {
	MatchupID int     `json:"matchup_id"` // teams sharing the same matchup_id played each other
	RosterID  int     `json:"roster_id"`
	Points    float64 `json:"points"`
}

// BracketEntry represents a single game in a playoff bracket
type BracketEntry struct {
	Round     int  `json:"r"`
	MatchupID int  `json:"m"`
	Winner    int  `json:"w"` // roster_id of the winner
	Loser     int  `json:"l"` // roster_id of the loser
	Place     *int `json:"p"` // final placement of the winner (e.g. 1 = champion); non-nil only on placement-determining games
}

// NFLState represents the current state of the NFL season on Sleeper
type NFLState struct {
	Week       int    `json:"week"`
	Season     string `json:"season"`
	SeasonType string `json:"season_type"` // "pre", "regular", "post"
}
