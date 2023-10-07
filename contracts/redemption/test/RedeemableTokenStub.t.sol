// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {BaseRedeemableToken} from "proof/redemption/voucher/BaseRedeemableToken.sol";

contract RedeemableTokenStub is BaseRedeemableToken {
    error AlreadyRedeemed(uint256 tokenId);

    mapping(address => mapping(uint256 => bool)) public approvals;
    mapping(uint256 => bool) public isRedeemed;

    constructor(address admin, address steerer) {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(DEFAULT_STEERING_ROLE, steerer);
    }

    function setSenderApproval(address sender, uint256 tokenId, bool toggle) external {
        approvals[sender][tokenId] = toggle;
    }

    function _isSenderAllowedToSpend(address sender, uint256 tokenId) internal view override returns (bool result) {
        return approvals[sender][tokenId];
    }

    function _doRedeem(address, uint256 tokenId) internal override {
        if (isRedeemed[tokenId]) {
            revert AlreadyRedeemed(tokenId);
        }

        isRedeemed[tokenId] = true;
    }
}
