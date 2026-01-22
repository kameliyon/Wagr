package auth

import (
	"time"
)

// User represents an authenticated user
type User struct {
	ID            string    `json:"id"`
	WalletAddress string    `json:"wallet_address"`
	Nonce         string    `json:"-"` // Never expose nonce in JSON responses
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NonceRequest is the request body for getting a nonce
type NonceRequest struct {
	WalletAddress string `json:"wallet_address"`
}

// NonceResponse is returned when requesting a nonce
type NonceResponse struct {
	Nonce   string `json:"nonce"`
	Message string `json:"message"` // Human-readable message to sign
}

// VerifyRequest is the request body for verifying a signature
type VerifyRequest struct {
	WalletAddress string `json:"wallet_address"`
	Signature     string `json:"signature"`     // Hex-encoded signature
	PublicKey     string `json:"public_key"`    // Hex-encoded public key (for Midnight)
}

// AuthResponse is returned after successful authentication
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Claims represents JWT claims
type Claims struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
