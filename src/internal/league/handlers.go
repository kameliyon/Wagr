package league

import (
	"encoding/json"
	"errors"
	"fmt"
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
		fmt.Println(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leagues)
}

// DeleteLeague handles DELETE /{leagueId} (requires authentication)
func (h *Handler) DeleteLeague(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	err := h.service.DeleteLeague(r.Context(), leagueID, claims.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetLeagueSettings handles GET /{leagueId}/settings (requires authentication)
func (h *Handler) GetLeagueSettings(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	settings, err := h.service.GetLeagueSettings(r.Context(), leagueID, claims.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// UpdateLeagueSettings handles PUT /{leagueId}/settings (requires authentication, commissioner only)
func (h *Handler) UpdateLeagueSettings(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.EntryFeeCents < 0 {
		http.Error(w, "entry_fee_cents must be non-negative", http.StatusBadRequest)
		return
	}
	settings, err := h.service.UpdateLeagueSettings(r.Context(), leagueID, claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrNotCommissioner) {
			http.Error(w, "forbidden: you are not the commissioner", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
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
