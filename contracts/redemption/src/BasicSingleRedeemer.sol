// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {IRedeemableToken} from "proof/redemption/interfaces/IRedeemableToken.sol";
import {ISingleRedeemer} from "proof/redemption/interfaces/ISingleRedeemer.sol";

interface BasicSingleRedeemerEvents {
    /**
     * @notice Emitted on redemption.
     */
    event VoucherRedeemed(address indexed sender, IRedeemableToken indexed voucher, uint256 tokenId);
}

/**
 * @notice Basic redeemer contract without any internal bookkeeping.
 */
contract BasicSingleRedeemer is ISingleRedeemer, BasicSingleRedeemerEvents {
    /**
     * @notice Redeems a voucher and emits an event as proof.
     */
    function redeem(IRedeemableToken voucher, uint256 tokenId) public virtual {
        emit VoucherRedeemed(msg.sender, voucher, tokenId);
        voucher.redeem(msg.sender, tokenId);
    }
}
