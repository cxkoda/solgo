// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {IRedeemableToken} from "proof/redemption/interfaces/IRedeemableToken.sol";
import {BasicSingleRedeemer} from "proof/redemption/BasicSingleRedeemer.sol";

/**
 * @notice Basic redeemer contract with internal bookkeeping.
 */
contract BasicSingleRecordedRedeemer is BasicSingleRedeemer {
    /**
     * @notice Keeps track of who redeemed which voucher.
     */
    mapping(address => mapping(IRedeemableToken => uint256[])) internal _redeemedVouchers;

    /**
     * @notice Redeems a voucher and emits an event as proof.
     */
    function redeem(IRedeemableToken voucher, uint256 tokenId) public virtual override {
        _redeemedVouchers[msg.sender][voucher].push(tokenId);
        super.redeem(voucher, tokenId);
    }

    /**
     * @notice Returns the number of vouchers redeemed by a given address.
     */
    function numVouchersRedeemed(address sender, IRedeemableToken voucher) public view returns (uint256) {
        return _redeemedVouchers[sender][voucher].length;
    }

    /**
     * @notice Returns the voucher tokenIds redeemed by a given address.
     */
    function redeemedVoucherIds(address sender, IRedeemableToken voucher) public view returns (uint256[] memory) {
        return _redeemedVouchers[sender][voucher];
    }

    /**
     * @notice  Returns the voucher tokenId redeemed by a given address at a
     * given index.
     */
    function redeemedVoucherIdAt(address sender, IRedeemableToken voucher, uint256 idx) public view returns (uint256) {
        return _redeemedVouchers[sender][voucher][idx];
    }
}
