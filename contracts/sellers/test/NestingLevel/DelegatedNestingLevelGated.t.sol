// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";
import {ProofTest} from "proof/constants/Testing.sol";
import {IDelegationRegistry, DelegationRegistry} from "delegation-registry/DelegationRegistry.sol";

import {ISellable, ImmutableSellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

import {MoonbirdNestingLevelGated} from "proof/sellers/mechanics/MoonbirdNestingLevelGated.sol";
import {NestingLevelLib} from "proof/sellers/mechanics/NestingLevelLib.sol";

import {DelegatedTokenApprovalChecker} from "proof/sellers/base/TokenApprovalChecker.sol";
import {
    AMoonbirdNestingLevelGatedTest,
    AMoonbirdNestingLevelGatedPurchaseWithDelegationTest
} from "./ANestingLevelGated.t.sol";

contract DelegatedMoonbirdNestingLevelGatedFake is
    MoonbirdNestingLevelGated,
    ImmutableSellableCallbacker,
    DelegatedTokenApprovalChecker,
    ProofTest
{
    constructor(IERC721 gatingToken, ISellable s, IDelegationRegistry r)
        MoonbirdNestingLevelGated(gatingToken, NestingLevelLib.NestingLevel.Diamond)
        ImmutableSellableCallbacker(s)
        DelegatedTokenApprovalChecker(r)
    {}

    function changeAllowlistSigners(address[] calldata rm, address[] calldata add) public {
        _changeAllowlistSigners(rm, add);
    }

    function _cost(uint64 num) internal view virtual override returns (uint256) {
        return uint256(num) * 1 ether;
    }
}

/**
 * @dev This runs all purchase tests defined in
 * - AMoonbirdNestingLevelGatedPurchaseTest
 * - AMoonbirdNestingLevelGatedPurchaseWithDelegationTest
 * Performing tests via ERC721-approved and delegated operators.
 */
contract DelegatedMoonbirdNestingLevelGatedFakeTest is AMoonbirdNestingLevelGatedTest {
    DelegatedMoonbirdNestingLevelGatedFake public impl;

    function setUp() public virtual override {
        impl = new DelegatedMoonbirdNestingLevelGatedFake(gatingToken, sellable, delegationRegistry);
        seller = impl;
        _changeAllowlistSigners(new address[](0), toAddresses([signer]));
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual override {
        impl.changeAllowlistSigners(rm, add);
    }
}

contract DelegatedMoonbirdNestingLevelGatedFake_PurchaseWithDelegationTest is
    AMoonbirdNestingLevelGatedPurchaseWithDelegationTest,
    DelegatedMoonbirdNestingLevelGatedFakeTest
{}
