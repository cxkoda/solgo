// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {IRedeemableToken} from "./IRedeemableToken.sol";

/**
 * @notice Interface for a contract that should allow users to redeem a given
 * voucher token.
 */
interface ISingleRedeemer {
    /**
     * @notice Redeems a given voucher.
     * @dev This MUST inform the voucher contract about the redemption by
     * calling its `redeem` method.
     */
    function redeem(IRedeemableToken token, uint256 tokenId) external;
}
