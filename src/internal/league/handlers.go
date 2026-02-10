package league

import (
	"encoding/json"
	"net/http"

	"wagr/src/internal/auth"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for league management
type Handler struct {
	service *Service
}

// NewHandler creates a new league handler
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// LinkPlatform handles POST /link-platform (requires authentication)
func (h *Handler) LinkPlatform(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req LinkPlatformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate platform
	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}
	if req.PlatformUsername == "" {
		http.Error(w, "platform_username is required", http.StatusBadRequest)
		return
	}

	profile, err := h.service.LinkPlatformProfile(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)
}

// ImportLeague handles POST /import (requires authentication)
func (h *Handler) ImportLeague(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ImportLeagueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}
	if req.PlatformLeagueID == "" {
		http.Error(w, "platform_league_id is required", http.StatusBadRequest)
		return
	}

	response, err := h.service.ImportLeague(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetUserLeagues handles GET / (requires authentication)
func (h *Handler) GetUserLeagues(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	leagues, err := h.service.GetUserLeagues(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leagues)
}

// GetLeague handles GET /{leagueId} (requires authentication)
func (h *Handler) GetLeague(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	leagueID := chi.URLParam(r, "leagueId")
	if leagueID == "" {
		http.Error(w, "league_id is required", http.StatusBadRequest)
		return
	}

	league, err := h.service.GetLeague(r.Context(), leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get league members
	members, err := h.service.GetLeagueMembers(r.Context(), leagueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"league":  league,
		"members": members,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
