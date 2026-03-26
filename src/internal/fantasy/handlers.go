package fantasy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for platform-agnostic fantasy operations
type Handler struct {
	platformService *PlatformService
}

// NewHandler creates a new fantasy handler
func NewHandler(platformService *PlatformService) *Handler {
	return &Handler{
		platformService: platformService,
	}
}

// GetUser handles GET /{platform}/user/{identifier}
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	platform := PlatformType(chi.URLParam(r, "platform"))
	identifier := chi.URLParam(r, "identifier")

	user, err := h.platformService.GetUser(r.Context(), platform, identifier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetUserLeagues handles GET /{platform}/user/{userId}/leagues
func (h *Handler) GetUserLeagues(w http.ResponseWriter, r *http.Request) {
	platform := PlatformType(chi.URLParam(r, "platform"))
	userID := chi.URLParam(r, "userId")
	sport := r.URL.Query().Get("sport")
	season := r.URL.Query().Get("season")

	if sport == "" {
		sport = "nfl"
	}
	if season == "" {
		season = strconv.Itoa(time.Now().Year())
	}

	leagues, err := h.platformService.GetUserLeagues(r.Context(), platform, userID, sport, season)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leagues)
}

// GetLeague handles GET /{platform}/league/{leagueId}
func (h *Handler) GetLeague(w http.ResponseWriter, r *http.Request) {
	platform := PlatformType(chi.URLParam(r, "platform"))
	leagueID := chi.URLParam(r, "leagueId")

	league, err := h.platformService.GetLeague(r.Context(), platform, leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(league)
}

// GetLeagueMembers handles GET /{platform}/league/{leagueId}/members
func (h *Handler) GetLeagueMembers(w http.ResponseWriter, r *http.Request) {
	platform := PlatformType(chi.URLParam(r, "platform"))
	leagueID := chi.URLParam(r, "leagueId")

	members, err := h.platformService.GetLeagueMembers(r.Context(), platform, leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// GetLeagueRosters handles GET /{platform}/league/{leagueId}/rosters
func (h *Handler) GetLeagueRosters(w http.ResponseWriter, r *http.Request) {
	platform := PlatformType(chi.URLParam(r, "platform"))
	leagueID := chi.URLParam(r, "leagueId")

	rosters, err := h.platformService.GetLeagueRosters(r.Context(), platform, leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rosters)
}

// ListPlatforms handles GET /platforms
func (h *Handler) ListPlatforms(w http.ResponseWriter, r *http.Request) {
	platforms := h.platformService.ListPlatforms()

	response := map[string]interface{}{
		"platforms": platforms,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
