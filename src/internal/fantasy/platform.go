package fantasy

import (
	"context"
)

// PlatformType represents the fantasy sports platform
type PlatformType string

const (
	PlatformSleeper PlatformType = "sleeper"
	PlatformESPN    PlatformType = "espn"
	PlatformYahoo   PlatformType = "yahoo"
)

// FantasyPlatform defines the interface that all fantasy sports platform adapters must implement
type FantasyPlatform interface {
	// Name returns the platform type (e.g., "sleeper", "espn", "yahoo")
	Name() PlatformType

	// GetUser fetches user information by username or user ID
	// identifier can be username or platform-specific user ID
	GetUser(ctx context.Context, identifier string) (*PlatformUser, error)

	// GetUserLeagues fetches all leagues for a user
	// sport is the sport type (e.g., "nfl", "nba")
	// season is the year (e.g., "2024")
	GetUserLeagues(ctx context.Context, userID string, sport string, season string) ([]PlatformLeague, error)

	// GetLeague fetches detailed information about a specific league
	GetLeague(ctx context.Context, leagueID string) (*PlatformLeague, error)

	// GetLeagueMembers fetches all members of a league with their profile information
	GetLeagueMembers(ctx context.Context, leagueID string) ([]PlatformMember, error)

	// GetLeagueRosters fetches all rosters for a league
	GetLeagueRosters(ctx context.Context, leagueID string) ([]PlatformRoster, error)

	// GetLeagueMatchups fetches all team scores for a given scoring week
	GetLeagueMatchups(ctx context.Context, leagueID string, week int) ([]PlatformMatchup, error)

	// GetFinalStandings fetches final placements from the winners bracket.
	// Only covers playoff bracket teams; losers bracket and non-bracket teams are not included.
	GetFinalStandings(ctx context.Context, leagueID string) ([]PlatformStanding, error)

	// GetCurrentWeek returns the current scoring week for the platform's primary sport
	GetCurrentWeek(ctx context.Context) (int, error)

	// RequiresAuth returns true if the platform requires OAuth or API key authentication
	RequiresAuth() bool

	// ValidateCredentials validates user credentials for authenticated platforms
	// Returns nil if credentials are valid, error otherwise
	ValidateCredentials(ctx context.Context, credentials *PlatformCredentials) error
}
