package league

import (
	"context"
	"encoding/binary"
	"fmt"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
	"golang.org/x/crypto/sha3"
)

// HederaClient wraps the Hiero SDK for submitting contract transactions as the operator.
type HederaClient struct {
	client     *hiero.Client
	contractID hiero.ContractID
}

// NewHederaClient creates a client configured with the operator key for signing transactions.
// network should be "testnet", "mainnet", or "previewnet".
func NewHederaClient(network, operatorID, operatorKey, contractID string) (*HederaClient, error) {
	var client *hiero.Client
	switch network {
	case "mainnet":
		client = hiero.ClientForMainnet()
	case "previewnet":
		client = hiero.ClientForPreviewnet()
	default:
		client = hiero.ClientForTestnet()
	}

	opID, err := hiero.AccountIDFromString(operatorID)
	if err != nil {
		return nil, fmt.Errorf("invalid operator ID %q: %w", operatorID, err)
	}

	opKey, err := hiero.PrivateKeyFromString(operatorKey)
	if err != nil {
		return nil, fmt.Errorf("invalid operator key: %w", err)
	}

	client.SetOperator(opID, opKey)

	cID, err := hiero.ContractIDFromString(contractID)
	if err != nil {
		return nil, fmt.Errorf("invalid contract ID %q: %w", contractID, err)
	}

	return &HederaClient{client: client, contractID: cID}, nil
}

// ExecuteDistributePayout calls distributePayout(bytes32,address[],uint256[]) on the escrow contract.
// amounts must be in 6-decimal USDC units (entry_fee_cents * 10_000).
// Returns the Hedera transaction ID string on success.
func (c *HederaClient) ExecuteDistributePayout(_ context.Context, leagueID [32]byte, recipients [][20]byte, amounts []int64) (string, error) {
	if len(recipients) != len(amounts) {
		return "", fmt.Errorf("recipients and amounts length mismatch: %d vs %d", len(recipients), len(amounts))
	}
	if len(recipients) == 0 {
		return "", fmt.Errorf("no recipients provided")
	}

	callData := encodeDistributePayoutCall(leagueID, recipients, amounts)

	resp, err := hiero.NewContractExecuteTransaction().
		SetContractID(c.contractID).
		SetGas(900_000).
		SetFunctionParameters(callData).
		Execute(c.client)
	if err != nil {
		return "", fmt.Errorf("failed to execute distributePayout: %w", err)
	}

	receipt, err := resp.GetReceipt(c.client)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != hiero.StatusSuccess {
		return "", fmt.Errorf("distributePayout transaction failed with status: %s", receipt.Status)
	}

	return resp.TransactionID.String(), nil
}

// encodeDistributePayoutCall manually ABI-encodes the call to:
//
//	distributePayout(bytes32 leagueId, address[] recipients, uint256[] amounts)
//
// Layout after the 4-byte selector:
//
//	[0:32]            leagueId (bytes32, static)
//	[32:64]           offset to recipients array = 96
//	[64:96]           offset to amounts array = 128 + N*32
//	[96:128]          len(recipients) = N
//	[128:128+N*32]    recipients (each 20-byte address right-aligned in 32 bytes)
//	[128+N*32:...]    len(amounts) = N
//	[160+N*32:...]    amounts (each uint64 right-aligned in 32 bytes)
func encodeDistributePayoutCall(leagueID [32]byte, recipients [][20]byte, amounts []int64) []byte {
	N := len(recipients)

	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte("distributePayout(bytes32,address[],uint256[])"))
	selector := hasher.Sum(nil)[:4]

	offsetRecipients := 96
	offsetAmounts := 96 + 32 + N*32 // recipients head (32) + N slots (N*32), then amounts start

	totalLen := 4 + 32 + 32 + 32 + 32 + N*32 + 32 + N*32
	buf := make([]byte, totalLen)
	pos := 0

	copy(buf[pos:], selector)
	pos += 4

	copy(buf[pos:], leagueID[:])
	pos += 32

	binary.BigEndian.PutUint64(buf[pos+24:], uint64(offsetRecipients))
	pos += 32

	binary.BigEndian.PutUint64(buf[pos+24:], uint64(offsetAmounts))
	pos += 32

	// recipients array: length then elements
	binary.BigEndian.PutUint64(buf[pos+24:], uint64(N))
	pos += 32
	for _, addr := range recipients {
		copy(buf[pos+12:], addr[:]) // 12 zero bytes + 20 address bytes
		pos += 32
	}

	// amounts array: length then elements
	binary.BigEndian.PutUint64(buf[pos+24:], uint64(N))
	pos += 32
	for _, amount := range amounts {
		binary.BigEndian.PutUint64(buf[pos+24:], uint64(amount))
		pos += 32
	}

	return buf
}
