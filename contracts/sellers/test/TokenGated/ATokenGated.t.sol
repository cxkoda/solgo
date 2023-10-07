// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721, ERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";

import {SellerTest} from "../SellerTest.sol";
import {ISellable, SellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

import {TokenGated} from "proof/sellers/mechanics/TokenGated.sol";
import {InternallyPriced, ExactInternallyPriced, ExactFixedPrice} from "proof/sellers/base/InternallyPriced.sol";

import {TokenUsageTracker} from "proof/sellers/base/TokenUsageTracker.sol";

contract ERC721Fake is ERC721("", "") {
    function mint(address to, uint256 tokenId) public {
        _mint(to, tokenId);
    }

    function mint(address to, uint256[] memory tokenIds) public {
        for (uint256 i; i < tokenIds.length; ++i) {
            mint(to, tokenIds[i]);
        }
    }
}

abstract contract ATokenGatedTest is SellerTest {
    ERC721Fake public immutable gatingToken;
    TokenGated public seller;

    constructor() {
        gatingToken = new ERC721Fake();
    }

    function setUp() public virtual;
}

abstract contract ATokenGatedPurchaseTest is ATokenGatedTest {
    struct TestCase {
        address caller;
        uint256[] tokenIds;
        uint256 msgValue;
    }

    modifier assertUsedTokensMarked(uint256[] memory tokenIds, bytes memory err) {
        bool[] memory usedBefore = seller.alreadyPurchasedWithTokens(tokenIds);
        assertEq(usedBefore.length, tokenIds.length);
        _;
        bool[] memory usedAfter = seller.alreadyPurchasedWithTokens(tokenIds);
        bool fails = err.length > 0;
        for (uint256 i; i < tokenIds.length; ++i) {
            assertEq(usedAfter[i], fails ? usedBefore[i] : true);
        }
    }

    function _test(TestCase memory tt, bytes memory err)
        internal
        assertUsedTokensMarked(tt.tokenIds, err)
        assertNumItemsIncreased(tt.caller, tt.tokenIds.length, err)
    {
        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            expectSellablePurchaseHandled(tt.msgValue, tt.caller, uint64(tt.tokenIds.length));
        }
        vm.deal(tt.caller, tt.msgValue);
        vm.prank(tt.caller);
        seller.purchase{value: tt.msgValue}(tt.tokenIds);
    }

    struct Fuzz {
        address owner;
        uint256 numTokens;
        uint128[20] tokenIdDeltas;
    }

    function _happyCase(Fuzz memory fuzz) internal virtual returns (TestCase memory) {
        fuzz.numTokens = bound(fuzz.numTokens, 0, fuzz.tokenIdDeltas.length);
        uint256[] memory tokenIds = new uint256[](fuzz.numTokens);

        if (fuzz.numTokens > 0) {
            tokenIds[0] = fuzz.tokenIdDeltas[0];
        }
        for (uint256 i = 1; i < fuzz.numTokens; ++i) {
            // + 1 to guarantee no duplicates
            tokenIds[i] = tokenIds[i - 1] + fuzz.tokenIdDeltas[i] + 1;
        }

        _assumeNotContract(fuzz.owner);
        gatingToken.mint(fuzz.owner, tokenIds);

        return TestCase({caller: fuzz.owner, tokenIds: tokenIds, msgValue: seller.cost(uint64(tokenIds.length))});
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testUseTokenTwiceInDifferentTx(Fuzz memory fuzz) public {
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.tokenIds.length > 0);
        _test(tt, "");
        _test(tt, abi.encodeWithSelector(TokenUsageTracker.TokenAlreadyUsedForPurchase.selector, tt.tokenIds[0]));
    }

    function testUseTokenTwiceInSameTx(Fuzz memory fuzz) public {
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.tokenIds.length >= 2);
        tt.tokenIds[1] = tt.tokenIds[0];

        _test(tt, abi.encodeWithSelector(TokenUsageTracker.TokenAlreadyUsedForPurchase.selector, tt.tokenIds[0]));
    }

    struct DifferentCallerFuzzParams {
        Fuzz token;
        address caller;
    }

    function _differentCallerCase(DifferentCallerFuzzParams memory fuzz) internal returns (TestCase memory) {
        // Excluding zero address because it is "approved" by default
        vm.assume(fuzz.caller != address(0));
        vm.assume(fuzz.caller != fuzz.token.owner);

        TestCase memory tt = _happyCase(fuzz.token);
        tt.caller = fuzz.caller;

        return tt;
    }

    function testNotAllowedToClaim(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);
        vm.assume(tt.tokenIds.length > 0);
        _test(
            tt,
            abi.encodeWithSelector(
                TokenUsageTracker.OperatorNotAllowedToPurchaseWithToken.selector, fuzz.caller, tt.tokenIds[0]
            )
        );
    }

    function testApproved(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        for (uint256 i; i < tt.tokenIds.length; ++i) {
            vm.prank(fuzz.token.owner);
            gatingToken.approve(tt.caller, tt.tokenIds[i]);
        }

        _test(tt, "");
    }

    function testApprovedForAll(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        vm.prank(fuzz.token.owner);
        gatingToken.setApprovalForAll(tt.caller, true);

        _test(tt, "");
    }
}
