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

    // leagueId => member EVM address => amount paid (6-decimal USDC)
    mapping(bytes32 => mapping(address => uint256)) public payments;

    // leagueId => total collected (used to validate distributePayout totals)
    mapping(bytes32 => uint256) public leagueTotals;

    event EntryFeePaid(bytes32 indexed leagueId, address indexed member, uint256 amount);
    event PayoutDistributed(bytes32 indexed leagueId, address indexed recipient, uint256 amount);

    constructor(address _usdc) {
        usdc = _usdc;
        owner = msg.sender;
        // Associate USDC with this contract so it can hold the token.
        // SUCCESS = 22, TOKEN_ALREADY_ASSOCIATED_TO_ACCOUNT = 194
        int64 rc = IHederaTokenService(0x0000000000000000000000000000000000000167)
            .associateToken(address(this), _usdc);
        require(rc == 22 || rc == 194, "USDC association failed");
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    function payEntryFee(bytes32 leagueId, uint256 amount) external {
        require(amount > 0, "amount must be > 0");
        // Update state before external call (checks-effects-interactions)
        payments[leagueId][msg.sender] += amount;
        leagueTotals[leagueId] += amount;
        bool ok = IERC20(usdc).transferFrom(msg.sender, address(this), amount);
        require(ok, "USDC transfer failed");
        emit EntryFeePaid(leagueId, msg.sender, amount);
    }

    function distributePayout(
        bytes32 leagueId,
        address[] calldata recipients,
        uint256[] calldata amounts
    ) external onlyOwner {
        require(recipients.length == amounts.length, "length mismatch");

        // Validate total payout does not exceed what was collected for this league
        uint256 total = 0;
        for (uint256 i = 0; i < amounts.length; i++) {
            total += amounts[i];
        }
        require(total <= leagueTotals[leagueId], "payout exceeds league escrow");

        // Deduct from league total before transfers (checks-effects-interactions)
        leagueTotals[leagueId] -= total;

        for (uint256 i = 0; i < recipients.length; i++) {
            // Associate recipient with USDC if not already — required on Hedera before any transfer
            // SUCCESS = 22, TOKEN_ALREADY_ASSOCIATED_TO_ACCOUNT = 194
            int64 rc = IHederaTokenService(0x0000000000000000000000000000000000000167)
                .associateToken(recipients[i], usdc);
            require(rc == 22 || rc == 194, "recipient association failed");

            bool ok = IERC20(usdc).transfer(recipients[i], amounts[i]);
            require(ok, "payout transfer failed");

            emit PayoutDistributed(leagueId, recipients[i], amounts[i]);
        }
    }
}
