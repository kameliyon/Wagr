package sleeper

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"wagr/src/internal/handlers"
)

// Handlers holds dependencies for Sleeper HTTP handlers
type Handlers struct {
	client *Client
}

// NewHandlers creates a new Handlers instance
func NewHandlers(client *Client) *Handlers {
	return &Handlers{client: client}
}

// RegisterRoutes registers all Sleeper routes on the given router
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/user/{username}", h.GetUser)
	r.Get("/user/{userId}/leagues", h.GetUserLeagues)
	r.Get("/league/{leagueId}", h.GetLeague)
	r.Get("/league/{leagueId}/teams", h.GetLeagueTeams)
}

// GetUser handles GET /api/sleeper/user/{username}
func (h *Handlers) GetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := h.client.GetUser(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	handlers.RespondJSON(w, user)
}

// GetUserLeagues handles GET /api/sleeper/user/{userId}/leagues
func (h *Handlers) GetUserLeagues(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	// Default to NFL and current season
	sport := r.URL.Query().Get("sport")
	if sport == "" {
		sport = "nfl"
	}

	season := r.URL.Query().Get("season")
	if season == "" {
		season = strconv.Itoa(time.Now().Year())
	}

	leagues, err := h.client.GetUserLeagues(userID, sport, season)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	handlers.RespondJSON(w, leagues)
}

// GetLeague handles GET /api/sleeper/league/{leagueId}
func (h *Handlers) GetLeague(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueId")

	league, err := h.client.GetLeague(leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	handlers.RespondJSON(w, league)
}

// GetLeagueTeams handles GET /api/sleeper/league/{leagueId}/teams
func (h *Handlers) GetLeagueTeams(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueId")

	teams, err := h.client.GetLeagueTeams(leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	handlers.RespondJSON(w, teams)
}
