package auth

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// verifyEVMSignature verifies an EIP-191 personal_sign signature and confirms
// the recovered address matches the claimed wallet address.
func verifyEVMSignature(message, signature, address string) (bool, error) {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	if len(sigBytes) != 65 {
		return false, fmt.Errorf("expected 65-byte signature, got %d", len(sigBytes))
	}

	// MetaMask sends v=27 or v=28; crypto.Ecrecover expects 0 or 1
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256(append([]byte(prefix), []byte(message)...))

	pubKeyBytes, err := crypto.Ecrecover(hash, sigBytes)
	if err != nil {
		return false, fmt.Errorf("ecrecover failed: %w", err)
	}

	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("invalid recovered public key: %w", err)
	}

	recovered := crypto.PubkeyToAddress(*pubKey)
	return strings.EqualFold(recovered.Hex(), address), nil
}
