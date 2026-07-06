package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"wagr/src/internal/handlers"
)

// Handlers holds dependencies for auth HTTP handlers
type Handlers struct {
	service *Service
}

// NewHandlers creates a new Handlers instance
func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

// RegisterRoutes registers all auth routes on the given router
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Post("/nonce", h.GetNonce)
	r.Post("/verify", h.VerifySignature)
	r.With(h.AuthMiddleware).Get("/me", h.GetCurrentUser)
	r.With(h.AuthMiddleware).Post("/logout", h.Logout)
}

// GetNonce handles POST /api/auth/nonce
func (h *Handlers) GetNonce(w http.ResponseWriter, r *http.Request) {
	var req NonceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.WalletAddress == "" {
		respondError(w, http.StatusBadRequest, "wallet_address is required")
		return
	}

	// Validate wallet type if provided (defaults to 'hedera' in service)
	if req.WalletType != "" && req.WalletType != "hedera" && req.WalletType != "evm" {
		respondError(w, http.StatusBadRequest, "wallet_type must be 'hedera' or 'evm'")
		return
	}

	resp, err := h.service.GetOrCreateNonce(r.Context(), req.WalletAddress, req.WalletType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate nonce")
		return
	}

	handlers.RespondJSON(w, resp)
}

// VerifySignature handles POST /api/auth/verify
func (h *Handlers) VerifySignature(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// if req.WalletAddress == "" || req.Signature == "" || req.PublicKey == "" {
	if req.Signature == "" {
		respondError(w, http.StatusBadRequest, "signature are required")
		return
	}

	// Validate wallet type if provided (defaults to 'hedera' in service)
	// if req.WalletType != "" && req.WalletType != "hedera" {
	// 	respondError(w, http.StatusBadRequest, "wallet_type must be 'hedera'")
	// 	return
	// }

	resp, err := h.service.VerifySignature(r.Context(), &req)
	if err != nil {
		switch err {
		case ErrUserNotFound:
			respondError(w, http.StatusNotFound, "user not found")
		case ErrInvalidSignature:
			respondError(w, http.StatusUnauthorized, "invalid signature")
		default:
			respondError(w, http.StatusInternalServerError, "authentication failed")
		}
		return
	}

	handlers.RespondJSON(w, resp)
}

// GetCurrentUser handles GET /api/auth/me
func (h *Handlers) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	handlers.RespondJSON(w, user)
}

// Logout handles POST /api/auth/logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	// For JWT-based auth, logout is handled client-side by discarding the token
	// In a production system, you might want to implement token blacklisting
	w.WriteHeader(http.StatusNoContent)
}

// AuthMiddleware validates JWT tokens and adds claims to context
func (h *Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			respondError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims, err := h.service.ValidateToken(parts[1])
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := SetClaimsInContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
