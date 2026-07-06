// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IERC20 {
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function transfer(address to, uint256 amount) external returns (bool);
}

interface IHederaTokenService {
    function associateToken(address account, address token) external returns (int64 responseCode);
}

contract LeagueEscrow {
    address public immutable usdc;
    address public immutable owner;

    // HTS precompile response codes (Hedera ResponseCodeEnum protobuf values)
    int64 private constant RC_SUCCESS = 22;
    int64 private constant RC_ALREADY_ASSOCIATED = 194;

    // leagueId => member EVM address => USDC amount paid (6-decimal)
    mapping(bytes32 => mapping(address => uint256)) public payments;
    mapping(bytes32 => uint256) public leagueTotals;

    // leagueId => member EVM address => HBAR amount paid (weibars = 10^-18 HBAR)
    mapping(bytes32 => mapping(address => uint256)) public hbarPayments;
    mapping(bytes32 => uint256) public hbarLeagueTotals;

    event EntryFeePaid(bytes32 indexed leagueId, address indexed member, uint256 amount);
    event PayoutDistributed(bytes32 indexed leagueId, address indexed recipient, uint256 amount);
    event RefundClaimed(bytes32 indexed leagueId, address indexed member, uint256 amount);

    event EntryFeeHBARPaid(bytes32 indexed leagueId, address indexed member, uint256 amount);
    event PayoutHBARDistributed(bytes32 indexed leagueId, address indexed recipient, uint256 amount);
    event RefundHBARClaimed(bytes32 indexed leagueId, address indexed member, uint256 amount);

    constructor(address _usdc) {
        usdc = _usdc;
        owner = msg.sender;
        // Associate USDC with this contract so it can hold the token.
        int64 rc = IHederaTokenService(0x0000000000000000000000000000000000000167)
            .associateToken(address(this), _usdc);
        require(rc == RC_SUCCESS || rc == RC_ALREADY_ASSOCIATED, "USDC association failed");
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    // ── USDC functions ────────────────────────────────────────────────────────

    function payEntryFee(bytes32 leagueId, uint256 amount) external {
        require(amount > 0, "amount must be > 0");
        payments[leagueId][msg.sender] += amount;
        leagueTotals[leagueId] += amount;
        bool ok = IERC20(usdc).transferFrom(msg.sender, address(this), amount);
        require(ok, "USDC transfer failed");
        emit EntryFeePaid(leagueId, msg.sender, amount);
    }

    function claimRefund(bytes32 leagueId) external {
        uint256 amount = payments[leagueId][msg.sender];
        require(amount > 0, "nothing to refund");
        payments[leagueId][msg.sender] = 0;
        leagueTotals[leagueId] -= amount;
        bool ok = IERC20(usdc).transfer(msg.sender, amount);
        require(ok, "refund transfer failed");
        emit RefundClaimed(leagueId, msg.sender, amount);
    }

    function distributePayout(
        bytes32 leagueId,
        address[] calldata recipients,
        uint256[] calldata amounts
    ) external onlyOwner {
        require(recipients.length == amounts.length, "length mismatch");
        uint256 total = 0;
        for (uint256 i = 0; i < amounts.length; i++) {
            total += amounts[i];
        }
        require(total <= leagueTotals[leagueId], "payout exceeds league escrow");
        leagueTotals[leagueId] -= total;
        for (uint256 i = 0; i < recipients.length; i++) {
            bool ok = IERC20(usdc).transfer(recipients[i], amounts[i]);
            require(ok, "payout transfer failed");
            emit PayoutDistributed(leagueId, recipients[i], amounts[i]);
        }
    }

    function emergencyWithdraw(address token, uint256 amount) external onlyOwner {
        bool ok = IERC20(token).transfer(owner, amount);
        require(ok, "emergency withdrawal failed");
    }

    // ── HBAR functions ───────────────────────────────────────────────────────

    // msg.value is in weibars (1 HBAR = 10^18 weibars), matching Ethereum's wei convention.
    function payEntryFeeHBAR(bytes32 leagueId) external payable {
        require(msg.value > 0, "must send HBAR");
        hbarPayments[leagueId][msg.sender] += msg.value;
        hbarLeagueTotals[leagueId] += msg.value;
        emit EntryFeeHBARPaid(leagueId, msg.sender, msg.value);
    }

    function claimRefundHBAR(bytes32 leagueId) external {
        uint256 amount = hbarPayments[leagueId][msg.sender];
        require(amount > 0, "nothing to refund");
        hbarPayments[leagueId][msg.sender] = 0;
        hbarLeagueTotals[leagueId] -= amount;
        (bool ok, ) = payable(msg.sender).call{value: amount}("");
        require(ok, "HBAR refund failed");
        emit RefundHBARClaimed(leagueId, msg.sender, amount);
    }

    function distributePayoutHBAR(
        bytes32 leagueId,
        address[] calldata recipients,
        uint256[] calldata amounts
    ) external onlyOwner {
        require(recipients.length == amounts.length, "length mismatch");
        uint256 total = 0;
        for (uint256 i = 0; i < amounts.length; i++) {
            total += amounts[i];
        }
        require(total <= hbarLeagueTotals[leagueId], "payout exceeds escrow");
        hbarLeagueTotals[leagueId] -= total;
        for (uint256 i = 0; i < recipients.length; i++) {
            (bool ok, ) = payable(recipients[i]).call{value: amounts[i]}("");
            require(ok, "HBAR payout failed");
            emit PayoutHBARDistributed(leagueId, recipients[i], amounts[i]);
        }
    }

    function emergencyWithdrawHBAR(uint256 amount) external onlyOwner {
        (bool ok, ) = payable(owner).call{value: amount}("");
        require(ok, "HBAR emergency withdrawal failed");
    }

    // Allow the contract to receive plain HBAR transfers (e.g. admin top-ups).
    receive() external payable {}
}
