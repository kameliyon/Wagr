// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// Planted at 0x167 in tests via hardhat_setCode.
// Returns SUCCESS (22) for all associateToken calls so LeagueEscrow can
// run on a local Hardhat EVM without the real Hedera precompile.
contract MockHTS {
    function associateToken(address, address) external pure returns (int64) {
        return 22; // RC_SUCCESS
    }
}
