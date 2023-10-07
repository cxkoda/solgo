// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721, ERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";

import {SellerTest} from "../SellerTest.sol";
import {ISellable, ImmutableSellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";
import {InternallyPriced, ExactInternallyPriced, ExactFixedPrice} from "proof/sellers/base/InternallyPriced.sol";
import {DefaultTokenGated} from "proof/sellers/mechanics/TokenGated.sol";
import {ATokenGatedTest, ATokenGatedPurchaseTest} from "./ATokenGated.t.sol";

contract TokenGatedFake is DefaultTokenGated, ExactFixedPrice, ImmutableSellableCallbacker {
    constructor(ISellable sellable_, uint256 price, IERC721 gatingToken)
        ImmutableSellableCallbacker(sellable_)
        ExactFixedPrice(price)
        DefaultTokenGated(gatingToken)
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

contract TokenGatedFakeTest is ATokenGatedTest {
    TokenGatedFake public impl;

    function setUp() public virtual override {
        impl = new TokenGatedFake(sellable, DEFAULT_PRICE, gatingToken);
        seller = impl;
    }
}

contract TokenGatedFake_PurchaseTest is TokenGatedFakeTest, ATokenGatedPurchaseTest {}
