package auth

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

type HederaKeyResponse struct {
	Key struct {
		Key  string `json:"key"`
		Type string `json:"_type"`
	} `json:"key"`
}

type HederaKeyInfo struct {
	PublicKeyHex string
	KeyType      string
}

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

	switch keyInfo.KeyType {
	case "ECDSA_SECP256K1":
		return verifyECDSASignature(keyInfo.PublicKeyHex, message, sigBytes)
	case "ED25519":
		return verifyED25519(keyInfo.PublicKeyHex, message, sigBytes)
	default:
		return false, fmt.Errorf("unsupported key type: %s", keyInfo.KeyType)
	}
}

// verifyECDSASignature verifies an ECDSA secp256k1 signature.
// Uses the Hedera personal-sign prefix + keccak256 (Ethereum-compatible).
func verifyECDSASignature(pubKeyHex string, message string, sigBytes []byte) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(pubKeyHex, "0x"))
	if err != nil {
		return false, fmt.Errorf("decoding public key: %w", err)
	}

	pubKey, err := crypto.DecompressPubkey(pubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("parsing public key: %w", err)
	}

	prefix := fmt.Sprintf("\x19Hedera Signed Message:\n%d", len([]rune(message)))
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

	prefix := fmt.Sprintf("\x19Hedera Signed Message:\n%d", len([]rune(message)))
	prefixed := append([]byte(prefix), message...)

	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), prefixed, sigBytes), nil
}

func getHederaPublicKey(accountId string, isMainnet bool) (HederaKeyInfo, error) {
	baseURL := "https://testnet.mirrornode.hedera.com"
	if isMainnet {
		baseURL = "https://mainnet.mirrornode.hedera.com"
	}

	url := fmt.Sprintf("%s/api/v1/accounts/%s", baseURL, accountId)
	fmt.Printf("Checking Mirror Node with this URL: %s", url)

	resp, err := http.Get(url) //nolint:gosec
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
		KeyType:      keyResp.Key.Type,
	}, nil
}

func normalizeSignature(sigBytes []byte) []byte {
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
