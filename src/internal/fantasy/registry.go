package fantasy

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages multiple fantasy platform implementations
type Registry struct {
	mu        sync.RWMutex
	platforms map[PlatformType]FantasyPlatform
}

// NewRegistry creates a new platform registry
func NewRegistry() *Registry {
	return &Registry{
		platforms: make(map[PlatformType]FantasyPlatform),
	}
}

// Register adds a platform implementation to the registry
func (r *Registry) Register(platform FantasyPlatform) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.platforms[platform.Name()] = platform
}

// Get retrieves a platform implementation by type
func (r *Registry) Get(platformType PlatformType) (FantasyPlatform, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platform, exists := r.platforms[platformType]
	if !exists {
		return nil, fmt.Errorf("platform %s not registered", platformType)
	}

	return platform, nil
}

// ListPlatforms returns all registered platform types
func (r *Registry) ListPlatforms() []PlatformType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platforms := make([]PlatformType, 0, len(r.platforms))
	for platformType := range r.platforms {
		platforms = append(platforms, platformType)
	}

	return platforms
}

// PlatformService provides high-level orchestration across all fantasy platforms
type PlatformService struct {
	registry *Registry
}

// NewPlatformService creates a new platform service
func NewPlatformService(registry *Registry) *PlatformService {
	return &PlatformService{
		registry: registry,
	}
}

// GetUser fetches a user from a specific platform
func (s *PlatformService) GetUser(ctx context.Context, platform PlatformType, identifier string) (*PlatformUser, error) {
	p, err := s.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return p.GetUser(ctx, identifier)
}

// GetUserLeagues fetches leagues for a user from a specific platform
func (s *PlatformService) GetUserLeagues(ctx context.Context, platform PlatformType, userID string, sport string, season string) ([]PlatformLeague, error) {
	p, err := s.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return p.GetUserLeagues(ctx, userID, sport, season)
}

// GetLeague fetches league details from a specific platform
func (s *PlatformService) GetLeague(ctx context.Context, platform PlatformType, leagueID string) (*PlatformLeague, error) {
	p, err := s.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return p.GetLeague(ctx, leagueID)
}

// GetLeagueMembers fetches all members of a league from a specific platform
func (s *PlatformService) GetLeagueMembers(ctx context.Context, platform PlatformType, leagueID string) ([]PlatformMember, error) {
	p, err := s.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return p.GetLeagueMembers(ctx, leagueID)
}

// GetLeagueRosters fetches all rosters for a league from a specific platform
func (s *PlatformService) GetLeagueRosters(ctx context.Context, platform PlatformType, leagueID string) ([]PlatformRoster, error) {
	p, err := s.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return p.GetLeagueRosters(ctx, leagueID)
}

// ListPlatforms returns all available platforms
func (s *PlatformService) ListPlatforms() []PlatformType {
	return s.registry.ListPlatforms()
}
