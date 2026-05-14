package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"math/big"
	"strings"

	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
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

type HederaKeyResponse struct {
	Key struct {
		Key string `json:"key"`
		Type string `json:"_type"`
	} `json:"key"`
}

type HederaKeyInfo struct {
	PublicKeyHex string
	KeyType string
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
	// Default to 'hedera'
	if walletType == "" {
		walletType = "hedera"
	}

	// Validate wallet type
	if walletType != "hedera" {
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
	// Default to 'hedera'
	// walletType := req.WalletType
	// if walletType == "" {
	walletType := "hedera"
	// }

	// Validate wallet type
	if walletType != "hedera" {
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

	// Verify the signature
	// message := fmt.Sprintf("Sign this message to authenticate with WAGR:\n\nNonce: %s", user.Nonce)

	valid, err := verifyHederaSignature(req.Message, req.Signature, req.WalletAddress)

	if err != nil || !valid {
		fmt.Printf("verify sig is: %t\n", valid)
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

// verifyHederaSignature verifies a signature from a Hedera wallet
// Hedera supports both Ed25519 and ECDSA (secp256k1) keys
func verifyHederaSignature(message, signature, address string) (bool, error) {
	keyInfo, err := getHederaPublicKey(address, false)
	if err != nil {
		return false, fmt.Errorf("error fetching public key: %w", err)
	}

	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	sigBytes = normalizeSignature(sigBytes)
	
	// Use key type from Mirror Node if provided
	switch keyInfo.KeyType {
	case "ECDSA_SECP256K1":
		return verifyECDSASignature(keyInfo.PublicKeyHex, message, sigBytes)

	case "ED25519":
		return verifyED25519(keyInfo.PublicKeyHex, message, sigBytes)
	default:
		return false, fmt.Errorf("unsupported key type: %s", keyInfo.KeyType)
	}
}

// verifyECDSASignature verifies an ECDSA signature using secp256k1 curve.
// Hedera ECDSA keys use keccak256 (Ethereum-compatible), not SHA-256.
func verifyECDSASignature(pubKeyHex string, message string, sigBytes []byte) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(pubKeyHex, "0x"))
    if err != nil {
        return false, fmt.Errorf("decoding public key: %w", err)
    }

    pubKey, err := crypto.DecompressPubkey(pubKeyBytes)
    if err != nil {
        return false, fmt.Errorf("parsing public key: %w", err)
    }

    prefix := fmt.Sprintf("\x19Hedera Signed Message:\n%d", len([]rune(string(message))))
    prefixed := append([]byte(prefix), message...)
    msgHash := crypto.Keccak256(prefixed)

    r := new(big.Int).SetBytes(sigBytes[:32])
    s := new(big.Int).SetBytes(sigBytes[32:])

    return ecdsa.Verify(pubKey, msgHash, r, s), nil
}

func verifyED25519(pubKeyHex string, message string, sigBytes []byte) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(pubKeyHex, "0x"))
	if err != nil {
		return false, fmt.Errorf("error decoding public key: %w", err)
	}

	if len(pubKeyBytes) != 32 {
		return false, fmt.Errorf("invalid ED25519 public key length: %d", len(pubKeyBytes))
	}

	if len(sigBytes) != 64 {
		return false, fmt.Errorf("invalid ED25519 signature length: %d", len(sigBytes))
	}

	prefix := fmt.Sprintf("\x19Hedera Signed Message:\n%d", len([]rune(string(message))))
	prefixed := append([]byte(prefix), message...)

	pubKey := ed25519.PublicKey(pubKeyBytes)
	return ed25519.Verify(pubKey, prefixed, sigBytes), nil
}

func getHederaPublicKey(accountId string, isMainnet bool) (HederaKeyInfo, error) {
	baseURL := "https://testnet.mirrornode.hedera.com"
	if isMainnet {
		baseURL = "https://mainnet.mirrornode.hedera.com"
	}

	url := fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, accountId)
	fmt.Printf("Checking Mirror Node with this URL: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return HederaKeyInfo{}, fmt.Errorf("mirror node request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return HederaKeyInfo{}, fmt.Errorf("reading mirror node response: %w", err)
	}

	var keyResp HederaKeyResponse 
	if err := json.Unmarshal(body, &keyResp); err != nil {
		return HederaKeyInfo{}, fmt.Errorf("parsing mirror node response error: %w", err)
	}

	if keyResp.Key.Type != "ECDSA_SECP256K1" && keyResp.Key.Type != "ED25519" {
		return HederaKeyInfo{}, fmt.Errorf("account key type is %s", keyResp.Key.Type)
	}

	return HederaKeyInfo{
		PublicKeyHex: keyResp.Key.Key,
		KeyType: keyResp.Key.Type,
	}, nil
}

func normalizeSignature(sigBytes []byte) []byte{
	if len(sigBytes) == 65 {
		sigBytes = sigBytes[:64]
	}

	r := sigBytes[:len(sigBytes)/2]
	s := sigBytes[len(sigBytes)/2:]

	rPad := make([]byte, 32)
	sPad := make([]byte, 32)

	copy(rPad[32-len(r):], r)
	copy(sPad[32-len(s):], s)

	return append(rPad, sPad...)
}

