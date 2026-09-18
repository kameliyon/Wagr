package league

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestNextLeagueStatus(t *testing.T) {
	tests := []struct {
		stored, platform string
		want             string
		wantUpdate       bool
	}{
		// NULL stored status behaves like pre_draft
		{"", "", "", false},
		{"", "pre_draft", "", false},
		{"", "drafting", "drafting", true},
		{"", "in_season", "in_season", true},
		{"", "post_season", "in_season", true},
		{"", "complete", "in_season", true},
		{"", "postponed", "", false},

		{"pre_draft", "", "pre_draft", false},
		{"pre_draft", "pre_draft", "pre_draft", false},
		{"pre_draft", "drafting", "drafting", true},
		{"pre_draft", "in_season", "in_season", true},
		{"pre_draft", "post_season", "in_season", true},
		{"pre_draft", "complete", "in_season", true},
		{"pre_draft", "postponed", "pre_draft", false},

		{"drafting", "", "drafting", false},
		{"drafting", "pre_draft", "drafting", false},
		{"drafting", "drafting", "drafting", false},
		{"drafting", "in_season", "in_season", true},
		{"drafting", "post_season", "in_season", true},
		{"drafting", "complete", "in_season", true},
		{"drafting", "postponed", "drafting", false},

		// A league stored as post_season is rewritten to in_season so the payout
		// jobs (which query only for in_season) can see it.
		{"post_season", "in_season", "in_season", true},
		{"post_season", "post_season", "in_season", true},
		{"post_season", "complete", "in_season", true},
		{"post_season", "pre_draft", "post_season", false},
		{"post_season", "drafting", "post_season", false},
		{"post_season", "postponed", "post_season", false},

		// in_season and complete are never advanced by the refresh
		{"in_season", "", "in_season", false},
		{"in_season", "pre_draft", "in_season", false},
		{"in_season", "drafting", "in_season", false},
		{"in_season", "in_season", "in_season", false},
		{"in_season", "post_season", "in_season", false},
		{"in_season", "complete", "in_season", false},
		{"in_season", "postponed", "in_season", false},

		{"complete", "", "complete", false},
		{"complete", "pre_draft", "complete", false},
		{"complete", "drafting", "complete", false},
		{"complete", "in_season", "complete", false},
		{"complete", "post_season", "complete", false},
		{"complete", "complete", "complete", false},
		{"complete", "postponed", "complete", false},

		// Unknown stored status is left alone
		{"weird", "in_season", "weird", false},
	}

	for _, tt := range tests {
		got, gotUpdate := nextLeagueStatus(tt.stored, tt.platform)
		if got != tt.want || gotUpdate != tt.wantUpdate {
			t.Errorf("nextLeagueStatus(%q, %q) = (%q, %v), want (%q, %v)",
				tt.stored, tt.platform, got, gotUpdate, tt.want, tt.wantUpdate)
		}
		if gotUpdate && got == "complete" {
			t.Errorf("nextLeagueStatus(%q, %q) wrote complete", tt.stored, tt.platform)
		}
	}
}

func TestFilterCurrentSeasonLeagues(t *testing.T) {
	var logBuf bytes.Buffer
	prevOutput := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(prevOutput) })

	leagues := []statusRefreshLeague{
		{ID: "l-current", Platform: "sleeper", Season: "2026", Status: "pre_draft"},
		{ID: "l-stale", Platform: "sleeper", Season: "2024", Status: "pre_draft"},
		{ID: "l-no-season-lookup", Platform: "espn", Season: "2026", Status: "pre_draft"},
	}

	kept := filterCurrentSeasonLeagues(leagues, map[string]string{"sleeper": "2026"})

	if len(kept) != 1 || kept[0].ID != "l-current" {
		t.Fatalf("kept = %+v, want only l-current", kept)
	}
	logs := logBuf.String()
	for _, id := range []string{"l-stale", "l-no-season-lookup"} {
		if !strings.Contains(logs, id) {
			t.Errorf("expected a log line naming skipped league %s; logs:\n%s", id, logs)
		}
	}
}

type statusUpdate struct {
	leagueID, oldStatus, newStatus string
}

func TestRefreshLeagueStatuses(t *testing.T) {
	var logBuf bytes.Buffer
	prevOutput := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(prevOutput) })

	leagues := []statusRefreshLeague{
		{ID: "l-started", Platform: "sleeper", PlatformLeagueID: "p1", Status: "pre_draft"},
		{ID: "l-error", Platform: "sleeper", PlatformLeagueID: "p2", Status: "pre_draft"},
		{ID: "l-complete", Platform: "sleeper", PlatformLeagueID: "p3", Status: "drafting"},
		{ID: "l-backward", Platform: "sleeper", PlatformLeagueID: "p4", Status: "drafting"},
		{ID: "l-null", Platform: "sleeper", PlatformLeagueID: "p5", Status: ""},
		{ID: "l-unknown", Platform: "sleeper", PlatformLeagueID: "p6", Status: "pre_draft"},
		{ID: "l-update-fails", Platform: "sleeper", PlatformLeagueID: "p7", Status: "pre_draft"},
		{ID: "l-playoffs", Platform: "sleeper", PlatformLeagueID: "p8", Status: "pre_draft"},
		{ID: "l-stored-postseason", Platform: "sleeper", PlatformLeagueID: "p9", Status: "post_season"},
		{ID: "l-contended", Platform: "sleeper", PlatformLeagueID: "p10", Status: "pre_draft"},
		{ID: "l-empty-status", Platform: "sleeper", PlatformLeagueID: "p11", Status: "pre_draft"},
	}
	platformStatus := map[string]string{
		"p1":  "in_season",
		"p3":  "complete",
		"p4":  "pre_draft",
		"p5":  "drafting",
		"p6":  "postponed",
		"p7":  "in_season",
		"p8":  "post_season",
		"p9":  "post_season",
		"p10": "in_season",
		"p11": "",
	}

	var fetched []string
	fetch := func(_ context.Context, platform, platformLeagueID string) (string, error) {
		fetched = append(fetched, platformLeagueID)
		if platformLeagueID == "p2" {
			return "", errors.New("sleeper timeout")
		}
		return platformStatus[platformLeagueID], nil
	}

	var updates []statusUpdate
	update := func(_ context.Context, leagueID, oldStatus, newStatus string) (bool, error) {
		if leagueID == "l-update-fails" {
			return false, errors.New("db down")
		}
		if leagueID == "l-contended" {
			return false, nil // another oracle run already moved it
		}
		updates = append(updates, statusUpdate{leagueID, oldStatus, newStatus})
		return true, nil
	}

	n := refreshLeagueStatuses(context.Background(), leagues, fetch, update)

	if len(fetched) != len(leagues) {
		t.Errorf("fetched %d leagues, want %d (errors must not stop the loop)", len(fetched), len(leagues))
	}

	want := []statusUpdate{
		{"l-started", "pre_draft", "in_season"},
		{"l-complete", "drafting", "in_season"},
		{"l-null", "", "drafting"},
		{"l-playoffs", "pre_draft", "in_season"},
		{"l-stored-postseason", "post_season", "in_season"},
	}
	if len(updates) != len(want) {
		t.Fatalf("updates = %+v, want %+v", updates, want)
	}
	for i := range want {
		if updates[i] != want[i] {
			t.Errorf("update[%d] = %+v, want %+v", i, updates[i], want[i])
		}
		if updates[i].newStatus == "complete" {
			t.Errorf("update[%d] wrote complete", i)
		}
	}
	// l-contended reported no row changed, so it must not count as updated.
	if n != len(want) {
		t.Errorf("updated count = %d, want %d", n, len(want))
	}

	logs := logBuf.String()
	for _, id := range []string{"l-error", "l-unknown", "l-update-fails", "l-contended", "l-empty-status"} {
		if !strings.Contains(logs, id) {
			t.Errorf("expected a log line naming league %s; logs:\n%s", id, logs)
		}
	}
}
