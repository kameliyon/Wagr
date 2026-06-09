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

func (h *Handler) RegisterRoutes(r chi.Router, auth auth.Handlers) {
		r.Use(auth.AuthMiddleware)
		r.Post("/link-platform", h.LinkPlatform)
		r.Post("/import", h.ImportLeague)
		r.Get("/", h.GetUserLeagues)
		r.Get("/{leagueId}", h.GetLeague)
		r.Delete("/{leagueId}", h.DeleteLeague)
		r.Get("/{leagueId}/settings", h.GetLeagueSettings)
		r.Put("/{leagueId}/settings", h.UpdateLeagueSettings)
		r.Post("/{leagueId}/payment-token", h.SetPaymentToken)
		r.Post("/{leagueId}/pay", h.InitiatePayment)
		r.Post("/{leagueId}/confirm-payment", h.ConfirmPayment)
		r.Get("/{leagueId}/payment-status", h.GetPaymentStatus)
		r.Post("/{leagueId}/cancel", h.CancelLeague)
		r.Post("/{leagueId}/reactivate", h.ReactivateLeague)
		r.Post("/{leagueId}/confirm-refund", h.ConfirmRefund)
		r.Post("/{leagueId}/oracle/week-results", h.OracleWeekResults)
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

// SetPaymentToken handles POST /{leagueId}/payment-token
func (h *Handler) SetPaymentToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	var req SetPaymentTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	err := h.service.SetPaymentToken(r.Context(), leagueID, claims.UserID, req.Token)
	if err != nil {
		if errors.Is(err, ErrNotLeagueMember) {
			http.Error(w, "not a member of this league", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// InitiatePayment handles POST /{leagueId}/pay — returns USDC escrow payment instructions
func (h *Handler) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	instructions, err := h.service.InitiatePayment(r.Context(), leagueID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotLeagueMember) {
			http.Error(w, "not a member of this league", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrAlreadyPaid) {
			http.Error(w, "already paid", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instructions)
}

// ConfirmPayment handles POST /{leagueId}/confirm-payment
func (h *Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")

	var req ConfirmPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TransactionID == "" {
		http.Error(w, "transaction_id is required", http.StatusBadRequest)
		return
	}

	err := h.service.ConfirmPayment(r.Context(), leagueID, claims.UserID, req.TransactionID)
	if err != nil {
		if errors.Is(err, ErrNotLeagueMember) {
			http.Error(w, "not a member of this league", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrAlreadyPaid) {
			http.Error(w, "already paid", http.StatusConflict)
			return
		}
		if errors.Is(err, ErrPaymentInsufficient) {
			http.Error(w, "on-chain payment not found or insufficient", http.StatusPaymentRequired)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPaymentStatus handles GET /{leagueId}/payment-status
func (h *Handler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	members, err := h.service.GetPaymentStatus(r.Context(), leagueID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotLeagueMember) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// CancelLeague handles POST /{leagueId}/cancel (commissioner only)
func (h *Handler) CancelLeague(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	err := h.service.CancelLeague(r.Context(), leagueID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotCommissioner) {
			http.Error(w, "forbidden: you are not the commissioner", http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrLeagueAlreadyCancelled) {
			http.Error(w, "league is already cancelled", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReactivateLeague handles POST /{leagueId}/reactivate (commissioner only)
func (h *Handler) ReactivateLeague(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")
	err := h.service.ReactivateLeague(r.Context(), leagueID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotCommissioner) {
			http.Error(w, "forbidden: you are not the commissioner", http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrLeagueNotCancelled) {
			http.Error(w, "league is not cancelled", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ConfirmRefund handles POST /{leagueId}/confirm-refund
func (h *Handler) ConfirmRefund(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	leagueID := chi.URLParam(r, "leagueId")

	var req ConfirmRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TransactionID == "" {
		http.Error(w, "transaction_id is required", http.StatusBadRequest)
		return
	}

	err := h.service.ConfirmRefund(r.Context(), leagueID, claims.UserID, req.TransactionID)
	if err != nil {
		if errors.Is(err, ErrNotLeagueMember) {
			http.Error(w, "not a member of this league", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrLeagueNotCancelled) {
			http.Error(w, "league is not cancelled", http.StatusConflict)
			return
		}
		if errors.Is(err, ErrPaymentInsufficient) {
			http.Error(w, "on-chain refund not confirmed -- it may still be propagating", http.StatusPaymentRequired)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OracleWeekResults is a placeholder for the Hedera Oracle integration.
// It will accept signed weekly score data and trigger on-chain bonus payouts.
func (h *Handler) OracleWeekResults(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "oracle integration not yet implemented", http.StatusNotImplemented)
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
