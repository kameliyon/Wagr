package league

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"wagr/src/internal/fantasy"
)

// validSeasonEndLeagueID is a syntactically valid UUID (32 hex chars once the
// dashes are stripped) so uuidToBytes32 succeeds.
const validSeasonEndLeagueID = "11111111-2222-3333-4444-555555555555"

type distributeCall struct {
	leagueID   [32]byte
	recipients [][20]byte
	amounts    []int64
}

type recordedPayout struct {
	leagueID    string
	rosterID    int
	place       int
	amountCents int64
	txHash      string
}

// fakeSeasonEnd is a hand-rolled fake for seasonEndDeps that records every call
// made against it, in order, so tests can assert both what ran and what didn't.
type fakeSeasonEnd struct {
	calls []string

	platformStatus    string
	platformStatusErr error

	standings    []fantasy.PlatformStanding
	standingsErr error

	members    []LeagueMember
	membersErr error

	// resolveEVMErrAll, if set, makes every resolveEVM call fail.
	resolveEVMErrAll error
	// resolveEVMErrFor lets a single wallet address fail resolution.
	resolveEVMErrFor map[string]error

	escrowBalance    int64
	escrowBalanceErr error

	distributeTxHash string
	distributeErr    error
	distributeCalls  []distributeCall

	claimResult bool
	claimErr    error

	markPayoutFailedErr error

	markPayoutExecutedResult bool
	markPayoutExecutedErr    error

	markSettledErr error

	recordedPayouts []recordedPayout
	recordErr       error
}

// fakeEVMAddr deterministically derives a fake EVM address from a wallet
// address string, so different wallets resolve to different addresses.
func fakeEVMAddr(seed string) [20]byte {
	var addr [20]byte
	copy(addr[:], seed)
	return addr
}

func (f *fakeSeasonEnd) deps() seasonEndDeps {
	return seasonEndDeps{
		platformStatus: func(ctx context.Context, platform, platformLeagueID string) (string, error) {
			f.calls = append(f.calls, "platformStatus")
			return f.platformStatus, f.platformStatusErr
		},
		finalStandings: func(ctx context.Context, platform, platformLeagueID string) ([]fantasy.PlatformStanding, error) {
			f.calls = append(f.calls, "finalStandings")
			return f.standings, f.standingsErr
		},
		loadMembers: func(ctx context.Context, leagueID string) ([]LeagueMember, error) {
			f.calls = append(f.calls, "loadMembers")
			return f.members, f.membersErr
		},
		resolveEVM: func(ctx context.Context, accountID string) ([20]byte, error) {
			f.calls = append(f.calls, "resolveEVM:"+accountID)
			if f.resolveEVMErrAll != nil {
				return [20]byte{}, f.resolveEVMErrAll
			}
			if err, ok := f.resolveEVMErrFor[accountID]; ok {
				return [20]byte{}, err
			}
			return fakeEVMAddr(accountID), nil
		},
		escrowBalance: func(ctx context.Context, leagueID [32]byte) (int64, error) {
			f.calls = append(f.calls, "escrowBalance")
			return f.escrowBalance, f.escrowBalanceErr
		},
		distribute: func(ctx context.Context, leagueID [32]byte, recipients [][20]byte, amounts []int64) (string, error) {
			f.calls = append(f.calls, "distribute")
			f.distributeCalls = append(f.distributeCalls, distributeCall{leagueID: leagueID, recipients: recipients, amounts: amounts})
			return f.distributeTxHash, f.distributeErr
		},
		markSettledWithoutPayout: func(ctx context.Context, leagueID string) error {
			f.calls = append(f.calls, "markSettledWithoutPayout")
			return f.markSettledErr
		},
		claimPayout: func(ctx context.Context, leagueID string) (bool, error) {
			f.calls = append(f.calls, "claimPayout")
			return f.claimResult, f.claimErr
		},
		markPayoutFailed: func(ctx context.Context, leagueID string) error {
			f.calls = append(f.calls, "markPayoutFailed")
			return f.markPayoutFailedErr
		},
		markPayoutExecuted: func(ctx context.Context, leagueID, txHash string) (bool, error) {
			f.calls = append(f.calls, "markPayoutExecuted")
			return f.markPayoutExecutedResult, f.markPayoutExecutedErr
		},
		recordMemberPayout: func(ctx context.Context, leagueID string, rosterID, place int, amountCents int64, txHash string) error {
			f.calls = append(f.calls, fmt.Sprintf("recordMemberPayout:%d", rosterID))
			f.recordedPayouts = append(f.recordedPayouts, recordedPayout{
				leagueID:    leagueID,
				rosterID:    rosterID,
				place:       place,
				amountCents: amountCents,
				txHash:      txHash,
			})
			return f.recordErr
		},
	}
}

// noWriteCalls are the deps that write to the leagues/league_members tables
// or that send the on-chain tx. Any test asserting "no writes" checks none of
// these appear in the call log.
var noWriteCalls = []string{
	"markSettledWithoutPayout",
	"claimPayout",
	"markPayoutFailed",
	"markPayoutExecuted",
	"distribute",
}

func assertNoneCalled(t *testing.T, calls []string, forbidden []string) {
	t.Helper()
	for _, c := range calls {
		for _, f := range forbidden {
			if c == f {
				t.Errorf("expected %q not to be called, but call log was %v", f, calls)
			}
		}
	}
}

func assertNotCalled(t *testing.T, calls []string, name string) {
	t.Helper()
	assertNoneCalled(t, calls, []string{name})
}

func assertCalledOnce(t *testing.T, calls []string, name string) {
	t.Helper()
	n := 0
	for _, c := range calls {
		if c == name {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected %q to be called exactly once, got %d (call log: %v)", name, n, calls)
	}
}

func onePlacementLeague() oracleLeague {
	return oracleLeague{
		ID:               validSeasonEndLeagueID,
		Platform:         "sleeper",
		PlatformLeagueID: "plat-1",
		PayoutStructure: []PayoutEntry{
			{Type: "placement", Place: 1, Label: "Champion", AmountCents: 100},
		},
	}
}

func withLogCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// 1. Platform status not "complete" -> nil, no DB writes, no distribute.
func TestSettleSeasonEnd_PlatformNotComplete_NoWrites(t *testing.T) {
	f := &fakeSeasonEnd{platformStatus: "in_season"}
	league := onePlacementLeague()

	err := settleSeasonEnd(context.Background(), league, f.deps())

	if err != nil {
		t.Fatalf("settleSeasonEnd() error = %v, want nil", err)
	}
	want := []string{"platformStatus"}
	if len(f.calls) != len(want) || f.calls[0] != want[0] {
		t.Errorf("calls = %v, want only %v", f.calls, want)
	}
}

// 2. Insufficient escrow -> ErrInsufficientEscrow, no writes at all. A later
// call with sufficient escrow claims, distributes, marks executed and records
// each winner's payout.
func TestSettleSeasonEnd_InsufficientEscrow_ThenSucceedsOnRetry(t *testing.T) {
	league := onePlacementLeague() // placement amount: 100 cents -> 1,000,000 micro-USDC
	f := &fakeSeasonEnd{
		platformStatus: "complete",
		standings:      []fantasy.PlatformStanding{{RosterID: 7, Place: 1}},
		members: []LeagueMember{
			{RosterID: 7, WalletAddress: "0.0.1007", DisplayName: "Alice"},
		},
		escrowBalance: 500_000, // below the 1,000,000 required
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if !errors.Is(err, ErrInsufficientEscrow) {
		t.Fatalf("settleSeasonEnd() error = %v, want ErrInsufficientEscrow", err)
	}
	assertNoneCalled(t, f.calls, noWriteCalls)
	if len(f.recordedPayouts) != 0 {
		t.Errorf("recordedPayouts = %v, want none", f.recordedPayouts)
	}

	// Second run: escrow is now funded.
	f.calls = nil
	f.escrowBalance = 2_000_000
	f.claimResult = true
	f.markPayoutExecutedResult = true
	f.distributeTxHash = "0xdeadbeef"

	err = settleSeasonEnd(context.Background(), league, f.deps())
	if err != nil {
		t.Fatalf("settleSeasonEnd() on retry error = %v, want nil", err)
	}
	assertCalledOnce(t, f.calls, "claimPayout")
	assertCalledOnce(t, f.calls, "distribute")
	assertCalledOnce(t, f.calls, "markPayoutExecuted")

	if len(f.recordedPayouts) != 1 {
		t.Fatalf("recordedPayouts = %v, want 1 entry", f.recordedPayouts)
	}
	got := f.recordedPayouts[0]
	if got.rosterID != 7 || got.place != 1 || got.amountCents != 100 || got.txHash != "0xdeadbeef" {
		t.Errorf("recordedPayouts[0] = %+v, want {rosterID:7 place:1 amountCents:100 txHash:0xdeadbeef}", got)
	}
}

// 3. Errors from platformStatus, finalStandings, loadMembers, escrowBalance
// propagate and write nothing.
func TestSettleSeasonEnd_UpstreamErrors_NoWrites(t *testing.T) {
	sentinel := errors.New("boom")

	tests := []struct {
		name    string
		mutate  func(f *fakeSeasonEnd)
		minCall string // the call that should have been attempted
	}{
		{
			name:    "platformStatus error",
			mutate:  func(f *fakeSeasonEnd) { f.platformStatusErr = sentinel },
			minCall: "platformStatus",
		},
		{
			name:    "finalStandings error",
			mutate:  func(f *fakeSeasonEnd) { f.standingsErr = sentinel },
			minCall: "finalStandings",
		},
		{
			name:    "loadMembers error",
			mutate:  func(f *fakeSeasonEnd) { f.membersErr = sentinel },
			minCall: "loadMembers",
		},
		{
			name: "escrowBalance error",
			mutate: func(f *fakeSeasonEnd) {
				f.standings = []fantasy.PlatformStanding{{RosterID: 1, Place: 1}}
				f.members = []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}}
				f.escrowBalanceErr = sentinel
			},
			minCall: "escrowBalance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeSeasonEnd{platformStatus: "complete"}
			tt.mutate(f)
			league := onePlacementLeague()

			err := settleSeasonEnd(context.Background(), league, f.deps())
			if err == nil {
				t.Fatalf("settleSeasonEnd() error = nil, want error")
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("settleSeasonEnd() error = %v, want it to wrap %v", err, sentinel)
			}
			found := false
			for _, c := range f.calls {
				if c == tt.minCall {
					found = true
				}
			}
			if !found {
				t.Errorf("expected %q to have been attempted; calls = %v", tt.minCall, f.calls)
			}
			assertNoneCalled(t, f.calls, noWriteCalls)
		})
	}
}

// 4. claimPayout returns (false, nil) -> nil, distribute not called, no other
// writes. claimPayout returns an error -> error returned, distribute not
// called.
func TestSettleSeasonEnd_ClaimLost_NoOp(t *testing.T) {
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus: "complete",
		standings:      []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:        []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:  10_000_000,
		claimResult:    false, // another run claimed it first
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if err != nil {
		t.Fatalf("settleSeasonEnd() error = %v, want nil", err)
	}
	assertCalledOnce(t, f.calls, "claimPayout")
	assertNotCalled(t, f.calls, "distribute")
	assertNoneCalled(t, f.calls, []string{"markSettledWithoutPayout", "markPayoutFailed", "markPayoutExecuted"})
}

func TestSettleSeasonEnd_ClaimError_ReturnsErrorAndSkipsDistribute(t *testing.T) {
	sentinel := errors.New("claim db error")
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus: "complete",
		standings:      []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:        []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:  10_000_000,
		claimErr:       sentinel,
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if !errors.Is(err, sentinel) {
		t.Fatalf("settleSeasonEnd() error = %v, want it to wrap %v", err, sentinel)
	}
	assertNotCalled(t, f.calls, "distribute")
}

// 5. Ordering: claimPayout happens before distribute.
func TestSettleSeasonEnd_ClaimHappensBeforeDistribute(t *testing.T) {
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus:           "complete",
		standings:                []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:                  []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:            10_000_000,
		claimResult:              true,
		markPayoutExecutedResult: true,
		distributeTxHash:         "0xabc",
	}

	if err := settleSeasonEnd(context.Background(), league, f.deps()); err != nil {
		t.Fatalf("settleSeasonEnd() error = %v, want nil", err)
	}

	claimIdx, distributeIdx := -1, -1
	for i, c := range f.calls {
		if c == "claimPayout" && claimIdx == -1 {
			claimIdx = i
		}
		if c == "distribute" && distributeIdx == -1 {
			distributeIdx = i
		}
	}
	if claimIdx == -1 || distributeIdx == -1 {
		t.Fatalf("expected both claimPayout and distribute in call log: %v", f.calls)
	}
	if claimIdx >= distributeIdx {
		t.Errorf("expected claimPayout (idx %d) before distribute (idx %d); calls = %v", claimIdx, distributeIdx, f.calls)
	}
}

// 6. distribute fails -> markPayoutFailed called, markPayoutExecuted NOT
// called, error returned. If markPayoutFailed also fails, a MANUAL CHECK log
// line naming the league is written.
func TestSettleSeasonEnd_DistributeFails_MarksFailed(t *testing.T) {
	sentinel := errors.New("tx rejected")
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus: "complete",
		standings:      []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:        []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:  10_000_000,
		claimResult:    true,
		distributeErr:  sentinel,
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if !errors.Is(err, sentinel) {
		t.Fatalf("settleSeasonEnd() error = %v, want it to wrap %v", err, sentinel)
	}
	assertCalledOnce(t, f.calls, "markPayoutFailed")
	assertNotCalled(t, f.calls, "markPayoutExecuted")
}

func TestSettleSeasonEnd_DistributeFails_MarkFailedAlsoFails_LogsManualCheck(t *testing.T) {
	logs := withLogCapture(t)
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus:      "complete",
		standings:           []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:             []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:       10_000_000,
		claimResult:         true,
		distributeErr:       errors.New("tx rejected"),
		markPayoutFailedErr: errors.New("db unreachable"),
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if err == nil {
		t.Fatalf("settleSeasonEnd() error = nil, want error")
	}
	if !strings.Contains(logs.String(), "MANUAL CHECK") || !strings.Contains(logs.String(), league.ID) {
		t.Errorf("expected log to contain MANUAL CHECK and league ID %s; logs:\n%s", league.ID, logs.String())
	}
}

// 7. distribute succeeds but markPayoutExecuted returns an error, or returns
// (false, nil) -> error returned, log contains "[oracle] MANUAL CHECK", the
// league ID and the tx hash; recordMemberPayout NOT called.
func TestSettleSeasonEnd_MarkExecutedError_LogsManualCheck(t *testing.T) {
	logs := withLogCapture(t)
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus:        "complete",
		standings:             []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:               []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:         10_000_000,
		claimResult:           true,
		distributeTxHash:      "0xfeedface",
		markPayoutExecutedErr: errors.New("db timeout"),
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if err == nil {
		t.Fatalf("settleSeasonEnd() error = nil, want error")
	}
	logStr := logs.String()
	for _, want := range []string{"[oracle] MANUAL CHECK", league.ID, "0xfeedface"} {
		if !strings.Contains(logStr, want) {
			t.Errorf("expected log to contain %q; logs:\n%s", want, logStr)
		}
	}
	if len(f.recordedPayouts) != 0 {
		t.Errorf("recordedPayouts = %v, want none", f.recordedPayouts)
	}
}

func TestSettleSeasonEnd_MarkExecutedNoRowUpdated_LogsManualCheck(t *testing.T) {
	logs := withLogCapture(t)
	league := onePlacementLeague()
	f := &fakeSeasonEnd{
		platformStatus:           "complete",
		standings:                []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
		members:                  []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
		escrowBalance:            10_000_000,
		claimResult:              true,
		distributeTxHash:         "0xfeedface",
		markPayoutExecutedResult: false, // no row matched; league no longer 'executing'
	}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if err == nil {
		t.Fatalf("settleSeasonEnd() error = nil, want error")
	}
	logStr := logs.String()
	for _, want := range []string{"[oracle] MANUAL CHECK", league.ID, "0xfeedface"} {
		if !strings.Contains(logStr, want) {
			t.Errorf("expected log to contain %q; logs:\n%s", want, logStr)
		}
	}
	if len(f.recordedPayouts) != 0 {
		t.Errorf("recordedPayouts = %v, want none", f.recordedPayouts)
	}
}

// 8. No placement rules, or placement rules with no eligible targets, both
// settle via a single markSettledWithoutPayout call with no claim/distribute.
func TestSettleSeasonEnd_NoPlacementRules_MarksSettledWithoutPayout(t *testing.T) {
	league := oracleLeague{
		ID:               validSeasonEndLeagueID,
		Platform:         "sleeper",
		PlatformLeagueID: "plat-1",
		PayoutStructure: []PayoutEntry{
			{Type: "weekly", BonusType: "weekly_high_score", AmountCents: 500},
		},
	}
	f := &fakeSeasonEnd{platformStatus: "complete"}

	err := settleSeasonEnd(context.Background(), league, f.deps())
	if err != nil {
		t.Fatalf("settleSeasonEnd() error = %v, want nil", err)
	}
	assertCalledOnce(t, f.calls, "markSettledWithoutPayout")
	assertNotCalled(t, f.calls, "claimPayout")
	assertNotCalled(t, f.calls, "distribute")
	// standings/members should never even be fetched, since there's nothing to pay.
	assertNotCalled(t, f.calls, "finalStandings")
	assertNotCalled(t, f.calls, "loadMembers")
}

func TestSettleSeasonEnd_NoEligibleTargets_MarksSettledWithoutPayout(t *testing.T) {
	tests := []struct {
		name          string
		standings     []fantasy.PlatformStanding
		members       []LeagueMember
		resolveAllErr error
	}{
		{
			name:      "member has no wallet address",
			standings: []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
			members:   []LeagueMember{{RosterID: 1, WalletAddress: "", DisplayName: "Bob"}},
		},
		{
			name:      "no member for the winning roster",
			standings: []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
			members:   []LeagueMember{{RosterID: 99, WalletAddress: "0.0.9999", DisplayName: "Someone Else"}},
		},
		{
			name:          "resolveEVM fails for every winner",
			standings:     []fantasy.PlatformStanding{{RosterID: 1, Place: 1}},
			members:       []LeagueMember{{RosterID: 1, WalletAddress: "0.0.1001", DisplayName: "Bob"}},
			resolveAllErr: errors.New("mirror node unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			league := onePlacementLeague()
			f := &fakeSeasonEnd{
				platformStatus:   "complete",
				standings:        tt.standings,
				members:          tt.members,
				resolveEVMErrAll: tt.resolveAllErr,
			}

			err := settleSeasonEnd(context.Background(), league, f.deps())
			if err != nil {
				t.Fatalf("settleSeasonEnd() error = %v, want nil", err)
			}
			assertCalledOnce(t, f.calls, "markSettledWithoutPayout")
			assertNotCalled(t, f.calls, "claimPayout")
			assertNotCalled(t, f.calls, "distribute")
		})
	}
}
