package sleeper

import (
	"context"
	"strconv"

	"wagr/src/internal/fantasy"
)

// Adapter wraps the Sleeper Client to implement the FantasyPlatform interface
type Adapter struct {
	client *Client
}

// NewAdapter creates a new Sleeper adapter
func NewAdapter(client *Client) *Adapter {
	return &Adapter{
		client: client,
	}
}

// Name returns the platform type
func (a *Adapter) Name() fantasy.PlatformType {
	return fantasy.PlatformSleeper
}

// GetUser fetches user information by username
func (a *Adapter) GetUser(ctx context.Context, identifier string) (*fantasy.PlatformUser, error) {
	user, err := a.client.GetUser(identifier)
	if err != nil {
		return nil, err
	}

	return &fantasy.PlatformUser{
		PlatformUserID: user.UserID,
		Username:       user.Username,
		DisplayName:    user.DisplayName,
		AvatarURL:      user.Avatar,
		Metadata:       make(map[string]string),
	}, nil
}

// GetUserLeagues fetches all leagues for a user
func (a *Adapter) GetUserLeagues(ctx context.Context, userID string, sport string, season string) ([]fantasy.PlatformLeague, error) {
	leagues, err := a.client.GetUserLeagues(userID, sport, season)
	if err != nil {
		return nil, err
	}

	platformLeagues := make([]fantasy.PlatformLeague, 0, len(leagues))
	for _, league := range leagues {
		platformLeagues = append(platformLeagues, convertLeague(&league))
	}

	return platformLeagues, nil
}

// GetLeague fetches detailed information about a specific league
func (a *Adapter) GetLeague(ctx context.Context, leagueID string) (*fantasy.PlatformLeague, error) {
	league, err := a.client.GetLeague(leagueID)
	if err != nil {
		return nil, err
	}

	platformLeague := convertLeague(league)
	return &platformLeague, nil
}

// GetLeagueMembers fetches all members of a league with their profile information
func (a *Adapter) GetLeagueMembers(ctx context.Context, leagueID string) ([]fantasy.PlatformMember, error) {
	// Fetch users — is_owner on each user identifies the league commissioner
	users, err := a.client.GetLeagueUsers(leagueID)
	if err != nil {
		return nil, err
	}

	// Fetch rosters to get each user's roster_id
	rosters, err := a.client.GetLeagueRosters(leagueID)
	if err != nil {
		return nil, err
	}

	// Build a map of owner_id -> roster_id
	rosterIDByOwner := make(map[string]int, len(rosters))
	for _, r := range rosters {
		rosterIDByOwner[r.OwnerID] = r.RosterID
	}

	members := make([]fantasy.PlatformMember, 0, len(users))
	for _, user := range users {
		members = append(members, fantasy.PlatformMember{
			PlatformUserID: user.UserID,
			Username:       user.UserID, // Sleeper doesn't expose username in the league users endpoint
			DisplayName:    user.DisplayName,
			AvatarURL:      user.Avatar,
			IsOwner:        user.IsOwner, // sourced from /league/{id}/users response
			RosterID:       rosterIDByOwner[user.UserID],
			Metadata:       user.Metadata,
		})
	}

	return members, nil
}

// GetLeagueRosters fetches all rosters for a league
func (a *Adapter) GetLeagueRosters(ctx context.Context, leagueID string) ([]fantasy.PlatformRoster, error) {
	rosters, err := a.client.GetLeagueRosters(leagueID)
	if err != nil {
		return nil, err
	}

	platformRosters := make([]fantasy.PlatformRoster, 0, len(rosters))
	for _, roster := range rosters {
		platformRosters = append(platformRosters, fantasy.PlatformRoster{
			RosterID:    roster.RosterID,
			OwnerID:     roster.OwnerID,
			Wins:        roster.Settings.Wins,
			Losses:      roster.Settings.Losses,
			Ties:        roster.Settings.Ties,
			TotalPoints: float64(roster.Settings.Fpts),
			Players:     roster.Players,
			Metadata: map[string]string{
				"league_id": roster.LeagueID,
			},
		})
	}

	return platformRosters, nil
}

// GetLeagueMatchups fetches all team scores for a given scoring week
func (a *Adapter) GetLeagueMatchups(ctx context.Context, leagueID string, week int) ([]fantasy.PlatformMatchup, error) {
	matchups, err := a.client.GetLeagueMatchups(leagueID, week)
	if err != nil {
		return nil, err
	}

	result := make([]fantasy.PlatformMatchup, 0, len(matchups))
	for _, m := range matchups {
		result = append(result, fantasy.PlatformMatchup{
			RosterID:  m.RosterID,
			MatchupID: m.MatchupID,
			Points:    m.Points,
			Week:      week,
		})
	}
	return result, nil
}

// GetFinalStandings fetches final placements from the winners bracket.
// Each entry with a non-nil Place field determines a placement: the winner gets that place,
// and the loser gets place+1 (e.g. championship game: winner=1st, loser=2nd).
func (a *Adapter) GetFinalStandings(ctx context.Context, leagueID string) ([]fantasy.PlatformStanding, error) {
	bracket, err := a.client.GetWinnersBracket(leagueID)
	if err != nil {
		return nil, err
	}

	standings := make([]fantasy.PlatformStanding, 0)
	for _, entry := range bracket {
		if entry.Place == nil {
			continue
		}
		if entry.Winner != 0 {
			standings = append(standings, fantasy.PlatformStanding{
				RosterID: entry.Winner,
				Place:    *entry.Place,
			})
		}
		if entry.Loser != 0 {
			standings = append(standings, fantasy.PlatformStanding{
				RosterID: entry.Loser,
				Place:    *entry.Place + 1,
			})
		}
	}
	return standings, nil
}

// GetCurrentWeek returns the current NFL scoring week from Sleeper's state endpoint
func (a *Adapter) GetCurrentWeek(ctx context.Context) (int, error) {
	state, err := a.client.GetNFLState()
	if err != nil {
		return 0, err
	}
	return state.Week, nil
}

// RequiresAuth returns false since Sleeper API doesn't require authentication
func (a *Adapter) RequiresAuth() bool {
	return false
}

// ValidateCredentials is a no-op for Sleeper since it doesn't require auth
func (a *Adapter) ValidateCredentials(ctx context.Context, credentials *fantasy.PlatformCredentials) error {
	return nil
}

// convertLeague converts a Sleeper League to a platform-agnostic PlatformLeague
func convertLeague(league *League) fantasy.PlatformLeague {
	// Determine scoring type from settings
	scoringType := "unknown"
	// Sleeper uses different fields for scoring, would need to check league.Settings for exact type
	// For now, we'll store it in metadata

	metadata := map[string]string{
		"playoff_week_start": strconv.Itoa(league.Settings.PlayoffWeekStart),
		"league_type":        strconv.Itoa(league.Settings.LeagueType),
	}

	return fantasy.PlatformLeague{
		PlatformLeagueID: league.LeagueID,
		Name:             league.Name,
		Sport:            league.Sport,
		Season:           league.Season,
		Status:           league.Status,
		TotalRosters:     league.TotalRosters,
		ScoringType:      scoringType,
		Metadata:         metadata,
	}
}
