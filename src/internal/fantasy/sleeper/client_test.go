package sleeper

import (
	"testing"
)

// These are integration tests that hit the real Sleeper API.
// Use a known public user for testing.

func TestGetUser(t *testing.T) {
	client := NewClient()

	user, err := client.GetUser("sleeperbot")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if user.Username != "sleeperbot" {
		t.Errorf("expected username 'sleeperbot', got '%s'", user.Username)
	}

	if user.UserID == "" {
		t.Error("expected non-empty user_id")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	client := NewClient()

	_, err := client.GetUser("this_user_definitely_does_not_exist_12345")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestGetUserLeagues(t *testing.T) {
	client := NewClient()

	// First get a user to get their ID
	user, err := client.GetUser("sleeperbot")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	leagues, err := client.GetUserLeagues(user.UserID, "nfl", "2024")
	if err != nil {
		t.Fatalf("GetUserLeagues failed: %v", err)
	}

	// sleeperbot may or may not have leagues, just verify no error
	t.Logf("Found %d leagues for sleeperbot in 2024", len(leagues))
}

func TestGetLeague(t *testing.T) {
	// Skip if we don't have a known league ID
	t.Skip("Skipping - need a known valid league ID for this test")
}

func TestGetLeagueTeams(t *testing.T) {
	// Skip if we don't have a known league ID
	t.Skip("Skipping - need a known valid league ID for this test")
}
