package sleeper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.sleeper.app/v1"
	defaultTimeout = 10 * time.Second
)

// Client handles communication with the Sleeper API
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Sleeper API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
	}
}

// GetUser fetches a user by username
func (c *Client) GetUser(username string) (*User, error) {
	url := fmt.Sprintf("%s/user/%s", c.baseURL, username)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var user *User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	// Sleeper returns null (decoded as nil) for non-existent users
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return user, nil
}

// GetUserLeagues fetches all leagues for a user in a given sport and season
func (c *Client) GetUserLeagues(userID, sport, season string) ([]League, error) {
	url := fmt.Sprintf("%s/user/%s/leagues/%s/%s", c.baseURL, userID, sport, season)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leagues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var leagues []League
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		return nil, fmt.Errorf("failed to decode leagues response: %w", err)
	}

	return leagues, nil
}

// GetLeague fetches a single league by ID
func (c *Client) GetLeague(leagueID string) (*League, error) {
	url := fmt.Sprintf("%s/league/%s", c.baseURL, leagueID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("league not found: %s", leagueID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var league League
	if err := json.NewDecoder(resp.Body).Decode(&league); err != nil {
		return nil, fmt.Errorf("failed to decode league response: %w", err)
	}

	return &league, nil
}

// GetLeagueUsers fetches all users in a league
func (c *Client) GetLeagueUsers(leagueID string) ([]LeagueUser, error) {
	url := fmt.Sprintf("%s/league/%s/users", c.baseURL, leagueID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var users []LeagueUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode league users response: %w", err)
	}

	return users, nil
}

// GetLeagueRosters fetches all rosters in a league
func (c *Client) GetLeagueRosters(leagueID string) ([]Roster, error) {
	url := fmt.Sprintf("%s/league/%s/rosters", c.baseURL, leagueID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rosters: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rosters []Roster
	if err := json.NewDecoder(resp.Body).Decode(&rosters); err != nil {
		return nil, fmt.Errorf("failed to decode rosters response: %w", err)
	}

	return rosters, nil
}

// GetLeagueMatchups fetches all matchup results for a given week
func (c *Client) GetLeagueMatchups(leagueID string, week int) ([]Matchup, error) {
	url := fmt.Sprintf("%s/league/%s/matchups/%d", c.baseURL, leagueID, week)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch matchups: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var matchups []Matchup
	if err := json.NewDecoder(resp.Body).Decode(&matchups); err != nil {
		return nil, fmt.Errorf("failed to decode matchups response: %w", err)
	}

	return matchups, nil
}

// GetWinnersBracket fetches the playoff winners bracket for a league
func (c *Client) GetWinnersBracket(leagueID string) ([]BracketEntry, error) {
	url := fmt.Sprintf("%s/league/%s/winners_bracket", c.baseURL, leagueID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch winners bracket: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var bracket []BracketEntry
	if err := json.NewDecoder(resp.Body).Decode(&bracket); err != nil {
		return nil, fmt.Errorf("failed to decode winners bracket response: %w", err)
	}

	return bracket, nil
}

// GetNFLState fetches the current NFL season/week state
func (c *Client) GetNFLState() (*NFLState, error) {
	url := fmt.Sprintf("%s/state/nfl", c.baseURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NFL state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var state NFLState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode NFL state response: %w", err)
	}

	return &state, nil
}

// GetLeagueTeams fetches combined team data (rosters + user info) for a league
func (c *Client) GetLeagueTeams(leagueID string) ([]Team, error) {
	users, err := c.GetLeagueUsers(leagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	rosters, err := c.GetLeagueRosters(leagueID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rosters: %w", err)
	}

	// Build a map of user_id -> user for quick lookup
	userMap := make(map[string]LeagueUser)
	for _, u := range users {
		userMap[u.UserID] = u
	}

	// Combine roster and user data
	teams := make([]Team, 0, len(rosters))
	for _, r := range rosters {
		team := Team{
			RosterID: r.RosterID,
			OwnerID:  r.OwnerID,
			Players:  r.Players,
			Wins:     r.Settings.Wins,
			Losses:   r.Settings.Losses,
			Ties:     r.Settings.Ties,
		}

		if user, ok := userMap[r.OwnerID]; ok {
			team.DisplayName = user.DisplayName
			team.Avatar = user.Avatar
		}

		teams = append(teams, team)
	}

	return teams, nil
}
