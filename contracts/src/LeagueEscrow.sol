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

    // leagueId => member EVM address => amount paid (6-decimal USDC)
    mapping(bytes32 => mapping(address => uint256)) public payments;

    // leagueId => total collected (used to validate distributePayout totals)
    mapping(bytes32 => uint256) public leagueTotals;

    event EntryFeePaid(bytes32 indexed leagueId, address indexed member, uint256 amount);
    event PayoutDistributed(bytes32 indexed leagueId, address indexed recipient, uint256 amount);
    event RefundClaimed(bytes32 indexed leagueId, address indexed member, uint256 amount);

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

    function payEntryFee(bytes32 leagueId, uint256 amount) external {
        require(amount > 0, "amount must be > 0");
        // checks-effects-interactions: update state before external call
        payments[leagueId][msg.sender] += amount;
        leagueTotals[leagueId] += amount;
        bool ok = IERC20(usdc).transferFrom(msg.sender, address(this), amount);
        require(ok, "USDC transfer failed");
        emit EntryFeePaid(leagueId, msg.sender, amount);
    }

    // Callers must already have USDC associated with their account (guaranteed for anyone who paid).
    // associateToken(msg.sender, ...) is not called here because the HTS precompile rejects
    // contract-initiated associations for arbitrary accounts even when msg.sender matches.
    function claimRefund(bytes32 leagueId) external {
        uint256 amount = payments[leagueId][msg.sender];
        require(amount > 0, "nothing to refund");
        // checks-effects-interactions: clear state before external calls
        payments[leagueId][msg.sender] = 0;
        leagueTotals[leagueId] -= amount;
        bool ok = IERC20(usdc).transfer(msg.sender, amount);
        require(ok, "refund transfer failed");
        emit RefundClaimed(leagueId, msg.sender, amount);
    }

    // Recipients must already have USDC associated (guaranteed for members who paid entry fees).
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

        // checks-effects-interactions: deduct before transfers
        leagueTotals[leagueId] -= total;

        for (uint256 i = 0; i < recipients.length; i++) {
            bool ok = IERC20(usdc).transfer(recipients[i], amounts[i]);
            require(ok, "payout transfer failed");
            emit PayoutDistributed(leagueId, recipients[i], amounts[i]);
        }
    }

    // Recover any tokens held by this contract. Safety valve for stranded funds.
    function emergencyWithdraw(address token, uint256 amount) external onlyOwner {
        bool ok = IERC20(token).transfer(owner, amount);
        require(ok, "emergency withdrawal failed");
    }
}
