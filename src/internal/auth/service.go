package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrInvalidNonce      = errors.New("invalid or expired nonce")
	ErrInvalidToken      = errors.New("invalid token")
)

// Service handles authentication business logic
type Service struct {
	db        *pgxpool.Pool
	jwtSecret []byte
}

// NewService creates a new auth service
func NewService(db *pgxpool.Pool) *Service {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "wagr-dev-secret-change-in-production"
	}
	return &Service{
		db:        db,
		jwtSecret: []byte(secret),
	}
}

// GetOrCreateNonce gets an existing user's nonce or creates a new user with a nonce
func (s *Service) GetOrCreateNonce(ctx context.Context, walletAddress, walletType string) (*NonceResponse, error) {
	// Default to 'midnight' for backwards compatibility during transition
	if walletType == "" {
		walletType = "midnight"
	}

	// Validate wallet type
	if walletType != "midnight" && walletType != "hedera" {
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Upsert user with new nonce
	query := `
		INSERT INTO users (wallet_address, wallet_type, nonce)
		VALUES ($1, $2, $3)
		ON CONFLICT (wallet_address)
		DO UPDATE SET nonce = $3, wallet_type = $2, updated_at = NOW()
		RETURNING id
	`
	var userID string
	err = s.db.QueryRow(ctx, query, walletAddress, walletType, nonce).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	message := fmt.Sprintf("Sign this message to authenticate with WAGR:\n\nNonce: %s", nonce)

	return &NonceResponse{
		Nonce:   nonce,
		Message: message,
	}, nil
}

// VerifySignature verifies the wallet signature and returns a JWT if valid
func (s *Service) VerifySignature(ctx context.Context, req *VerifyRequest) (*AuthResponse, error) {
	// Default to 'midnight' for backwards compatibility during transition
	walletType := req.WalletType
	if walletType == "" {
		walletType = "midnight"
	}

	// Validate wallet type
	if walletType != "midnight" && walletType != "hedera" {
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	// Get user and their current nonce
	var user User
	query := `
		SELECT id, wallet_address, wallet_type, nonce, created_at, updated_at
		FROM users
		WHERE wallet_address = $1
	`
	err := s.db.QueryRow(ctx, query, req.WalletAddress).Scan(
		&user.ID, &user.WalletAddress, &user.WalletType, &user.Nonce, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Verify the signature based on wallet type
	message := fmt.Sprintf("Sign this message to authenticate with WAGR:\n\nNonce: %s", user.Nonce)
	var valid bool

	switch walletType {
	case "midnight":
		valid, err = verifyMidnightSignature(req.PublicKey, message, req.Signature)
	case "hedera":
		valid, err = verifyHederaSignature(req.PublicKey, message, req.Signature, req.KeyType)
	default:
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	if err != nil || !valid {
		fmt.Println("Signature verification error:", err)
		return nil, ErrInvalidSignature
	}

	// Rotate the nonce to prevent replay attacks
	newNonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new nonce: %w", err)
	}

	_, err = s.db.Exec(ctx, "UPDATE users SET nonce = $1, updated_at = NOW() WHERE id = $2", newNonce, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update nonce: %w", err)
	}

	// Generate JWT
	token, err := s.generateJWT(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetUserByID retrieves a user by their ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var user User
	query := `
		SELECT id, wallet_address, wallet_type, nonce, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	err := s.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.WalletAddress, &user.WalletType, &user.Nonce, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// ValidateToken validates a JWT and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		walletType, _ := claims["wallet_type"].(string)
		if walletType == "" {
			walletType = "midnight" // Default for backwards compatibility
		}
		return &Claims{
			UserID:        claims["user_id"].(string),
			WalletAddress: claims["wallet_address"].(string),
			WalletType:    walletType,
		}, nil
	}

	return nil, ErrInvalidToken
}

// generateJWT creates a new JWT for the user
func (s *Service) generateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":        user.ID,
		"wallet_address": user.WalletAddress,
		"wallet_type":    user.WalletType,
		"exp":            time.Now().Add(24 * time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateNonce creates a cryptographically secure random nonce
func generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// verifyMidnightSignature verifies an Ed25519 signature from a Midnight wallet
func verifyMidnightSignature(publicKeyHex, message, signatureHex string) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %w", err)
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key length: expected %d, got %d", ed25519.PublicKeySize, len(pubKeyBytes))
	}

	if len(sigBytes) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature length: expected %d, got %d", ed25519.SignatureSize, len(sigBytes))
	}

	return ed25519.Verify(pubKeyBytes, []byte(message), sigBytes), nil
}

// verifyHederaSignature verifies a signature from a Hedera wallet
// Hedera supports both Ed25519 and ECDSA (secp256k1) keys
func verifyHederaSignature(publicKeyHex, message, signatureHex, keyType string) (bool, error) {
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %w", err)
	}

	// Use key type from Mirror Node if provided
	switch keyType {
	case "ECDSA_SECP256K1":
		return verifyECDSASignature(pubKeyBytes, message, sigBytes)

	case "ED25519":
		if len(pubKeyBytes) != ed25519.PublicKeySize {
			return false, fmt.Errorf("invalid Ed25519 public key length: expected %d, got %d", ed25519.PublicKeySize, len(pubKeyBytes))
		}
		if len(sigBytes) != ed25519.SignatureSize {
			return false, fmt.Errorf("invalid Ed25519 signature length: expected %d, got %d", ed25519.SignatureSize, len(sigBytes))
		}
		// Hedera wallets sign messages with a prefix (similar to EIP-191)
		// Format: \x19Hedera Signed Message:\n{length}{message}
		messageBytes := []byte(message)
		hederaMsg := fmt.Appendf(nil, "\x19Hedera Signed Message:\n%d%s", len(messageBytes), message)
		return ed25519.Verify(pubKeyBytes, hederaMsg, sigBytes), nil

	default:
		// Infer from key/signature length as fallback
		if len(sigBytes) == ed25519.SignatureSize && len(pubKeyBytes) == ed25519.PublicKeySize {
			// Try Hedera prefix format first
			messageBytes := []byte(message)
			hederaMsg := fmt.Appendf(nil, "\x19Hedera Signed Message:\n%d%s", len(messageBytes), message)
			if ed25519.Verify(pubKeyBytes, hederaMsg, sigBytes) {
				return true, nil
			}
			// Fallback to raw message
			return ed25519.Verify(pubKeyBytes, messageBytes, sigBytes), nil
		}
		// Assume ECDSA for non-64-byte signatures or non-32-byte keys
		return verifyECDSASignature(pubKeyBytes, message, sigBytes)
	}
}

// verifyECDSASignature verifies an ECDSA signature using secp256k1 curve
func verifyECDSASignature(pubKeyBytes []byte, message string, sigBytes []byte) (bool, error) {
	// Parse the public key using btcec (secp256k1)
	pubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse ECDSA public key: %w", err)
	}

	// Parse the signature - btcec can handle DER-encoded signatures directly
	sig, err := ecdsa.ParseDERSignature(sigBytes)
	if err != nil {
		// Try parsing as raw R||S format (64 or 65 bytes)
		if len(sigBytes) == 64 || len(sigBytes) == 65 {
			r := new(btcec.ModNScalar)
			r.SetByteSlice(sigBytes[:32])
			s := new(btcec.ModNScalar)
			s.SetByteSlice(sigBytes[32:64])
			sig = ecdsa.NewSignature(r, s)
		} else {
			return false, fmt.Errorf("failed to parse ECDSA signature (len=%d): %w", len(sigBytes), err)
		}
	}

	// Hash the message
	hash := sha256.Sum256([]byte(message))

	// Verify the signature
	return sig.Verify(hash[:], pubKey), nil
}
