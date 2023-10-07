// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {AccessControlEnumerable} from "ethier/utils/AccessControlEnumerable.sol";
import {RedeemableERC721ACommon} from "./voucher/RedeemableERC721ACommon.sol";

/**
 * @notice A redeemer that allows steerers to burn vouchers at will.
 * @dev Only for testing purposes.
 */
contract RoleGatedBurningRedeemer is AccessControlEnumerable {
    constructor(address admin, address steerer) {
        _setupRole(DEFAULT_ADMIN_ROLE, admin);
        _setupRole(DEFAULT_STEERING_ROLE, steerer);
    }

    /**
     * @notice Burns the given tokens.
     * @dev Usually a redeemer would give something in return, however, we're not doing that here since we're only
     * interested in the burn.
     */
    function burn(RedeemableERC721ACommon voucher, uint256[] calldata tokenIds)
        public
        onlyRole(DEFAULT_STEERING_ROLE)
    {
        for (uint256 i = 0; i < tokenIds.length; ++i) {
            voucher.redeem(voucher.ownerOf(tokenIds[i]), tokenIds[i]);
        }
    }
}
