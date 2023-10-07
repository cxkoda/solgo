// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721, ERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";
import {IDelegationRegistry, DelegationRegistry} from "delegation-registry/DelegationRegistry.sol";

import {SellerTest} from "../SellerTest.sol";
import {ISellable, ImmutableSellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";
import {InternallyPriced, ExactInternallyPriced, ExactFixedPrice} from "proof/sellers/base/InternallyPriced.sol";
import {
    DelegatedTokenGated,
    DelegatedTokenGatedSettablePrice
} from "proof/sellers/presets/DelegatedTokenGatedSettablePrice.sol";

import {ATokenGatedTest, ATokenGatedPurchaseTest} from "./ATokenGated.t.sol";

abstract contract ADelegatedTokenGatedTest is ATokenGatedTest {
    IDelegationRegistry public immutable delegationRegistry;

    constructor() {
        delegationRegistry = new DelegationRegistry();
    }
}

abstract contract ADelegatedPurchaseTest is ADelegatedTokenGatedTest, ATokenGatedPurchaseTest {
    function testDelegatedContractClaim(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        vm.prank(fuzz.token.owner);
        delegationRegistry.delegateForContract(tt.caller, address(gatingToken), true);

        _test(tt, "");
    }

    function testDelegatedTokenClaim(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        for (uint256 i; i < tt.tokenIds.length; ++i) {
            vm.prank(fuzz.token.owner);
            delegationRegistry.delegateForToken(tt.caller, address(gatingToken), tt.tokenIds[i], true);
        }

        _test(tt, "");
    }
}

contract DelegatedTokenGatedFake is DelegatedTokenGated, ExactFixedPrice, ImmutableSellableCallbacker {
    constructor(ISellable sellable_, uint256 price, IERC721 gatingToken, IDelegationRegistry registry)
        ImmutableSellableCallbacker(sellable_)
        ExactFixedPrice(price)
        DelegatedTokenGated(gatingToken, registry)
    {}

    function _checkAndModifyPurchase(address to, uint64 num, uint256 cost_, bytes memory data)
        internal
        view
        virtual
        override(InternallyPriced, ExactInternallyPriced)
        returns (address, uint64, uint256)
    {
        return ExactInternallyPriced._checkAndModifyPurchase(to, num, cost_, data);
    }
}

contract DelegatedTokenGatedFakeTest is ADelegatedTokenGatedTest {
    DelegatedTokenGatedFake public impl;

    function setUp() public virtual override {
        impl = new DelegatedTokenGatedFake(sellable,DEFAULT_PRICE, gatingToken, delegationRegistry);
        seller = impl;
    }
}

contract DelegatedTokenGatedFake_PurchaseTest is DelegatedTokenGatedFakeTest, ADelegatedPurchaseTest {}

contract DelegatedTokenGatedSettablePriceTest is ADelegatedTokenGatedTest {
    DelegatedTokenGatedSettablePrice public impl;

    function setUp() public virtual override {
        impl =
        new DelegatedTokenGatedSettablePrice(admin ,steerer, sellable,DEFAULT_PRICE, gatingToken ,delegationRegistry);
        seller = impl;
    }
}

contract DelegatedTokenGatedSettablePrice_PurchaseTest is
    DelegatedTokenGatedSettablePriceTest,
    ADelegatedPurchaseTest
{}
