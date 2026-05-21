package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidNonce     = errors.New("invalid or expired nonce")
	ErrInvalidToken     = errors.New("invalid token")
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
	if walletType == "" {
		walletType = "hedera"
	}

	if walletType != "hedera" {
		return nil, fmt.Errorf("unsupported wallet type: %s", walletType)
	}

	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

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

	valid, err := verifyHederaSignature(req.Message, req.Signature, req.WalletAddress)
	if err != nil || !valid {
		fmt.Printf("verify sig is: %t\n", valid)
		return nil, ErrInvalidSignature
	}

	newNonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new nonce: %w", err)
	}

	_, err = s.db.Exec(ctx, "UPDATE users SET nonce = $1, updated_at = NOW() WHERE id = $2", newNonce, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update nonce: %w", err)
	}

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
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
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
			walletType = "hedera"
		}
		return &Claims{
			UserID:        claims["user_id"].(string),
			WalletAddress: claims["wallet_address"].(string),
			WalletType:    walletType,
		}, nil
	}

	return nil, ErrInvalidToken
}

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

func generateNonce() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
