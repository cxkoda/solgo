// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";
import {IDelegationRegistry} from "delegation-registry/DelegationRegistry.sol";

import {ISellable, SellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

import {MoonbirdNestingLevelGated} from "proof/sellers/mechanics/MoonbirdNestingLevelGated.sol";
import {NestingLevelLib} from "proof/sellers/mechanics/NestingLevelLib.sol";

import {DelegatedMoonbirdNestingLevelGatedSettablePrice} from
    "proof/sellers/presets/DelegatedMoonbirdNestingLevelGatedSettablePrice.sol";
import {
    AMoonbirdNestingLevelGatedTest,
    AMoonbirdNestingLevelGatedPurchaseWithDelegationTest
} from "./ANestingLevelGated.t.sol";

contract DelegatedMoonbirdNestingLevelGatedSettablePriceTest is AMoonbirdNestingLevelGatedTest {
    DelegatedMoonbirdNestingLevelGatedSettablePrice public impl;

    function setUp() public virtual override {
        impl =
        new DelegatedMoonbirdNestingLevelGatedSettablePrice(admin, steerer,  sellable, DEFAULT_PRICE, gatingToken, NestingLevelLib.NestingLevel.Diamond, delegationRegistry);
        seller = impl;
        _changeAllowlistSigners(new address[](0), toAddresses([signer]));
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual override {
        vm.prank(steerer);
        impl.changeAllowlistSigners(rm, add);
    }
}

/**
 * @dev This runs all purchase tests defined in
 * - AMoonbirdNestingLevelGatedPurchaseTest
 * - AMoonbirdNestingLevelGatedPurchaseWithDelegationTest
 * Performing tests via ERC721-approved and delegated operators.
 */
contract DelegatedMoonbirdNestingLevelGatedSettablePrice_PurchaseWithDelegationTest is
    AMoonbirdNestingLevelGatedPurchaseWithDelegationTest,
    DelegatedMoonbirdNestingLevelGatedSettablePriceTest
{}

contract VandalTest is DelegatedMoonbirdNestingLevelGatedSettablePriceTest {
    function testCannotChangeSigners(address vandal, address[] memory rm, address[] memory add) public {
        vm.assume(!impl.hasRole(impl.DEFAULT_STEERING_ROLE(), vandal));
        vm.expectRevert(missingRoleError(vandal, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        impl.changeAllowlistSigners(rm, add);
    }
}
