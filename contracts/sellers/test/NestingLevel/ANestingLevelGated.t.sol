// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IERC721, ERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";
import {IDelegationRegistry, DelegationRegistry} from "delegation-registry/DelegationRegistry.sol";

import {SellerTest, ProofTest} from "../SellerTest.sol";
import {ISellable, SellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

import {InternallyPriced, ExactInternallyPriced, ExactFixedPrice} from "proof/sellers/base/InternallyPriced.sol";
import {TokenUsageTracker} from "proof/sellers/base/TokenUsageTracker.sol";

import {MoonbirdNestingLevelGated} from "proof/sellers/mechanics/MoonbirdNestingLevelGated.sol";
import {NestingLevelLib} from "proof/sellers/mechanics/NestingLevelLib.sol";

import {ERC721Fake} from "../TokenGated/ATokenGated.t.sol";

abstract contract AMoonbirdNestingLevelGatedTest is SellerTest {
    IDelegationRegistry public immutable delegationRegistry;

    ERC721Fake public immutable gatingToken;
    MoonbirdNestingLevelGated public seller;

    address public signer;
    uint256 public signerKey;

    constructor() {
        delegationRegistry = new DelegationRegistry();
        gatingToken = new ERC721Fake();
        (signer, signerKey) = makeAddrAndKey("signer");
    }

    function setUp() public virtual;

    function _sign(uint256 key, NestingLevelLib.MoonbirdNestingLevel memory payload)
        internal
        view
        returns (NestingLevelLib.SignedMoonbirdNestingLevel memory)
    {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(key, seller.digest(payload));
        return NestingLevelLib.SignedMoonbirdNestingLevel({payload: payload, signature: abi.encodePacked(r, s, v)});
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual;
}

abstract contract AMoonbirdNestingLevelGatedPurchaseTest is AMoonbirdNestingLevelGatedTest {
    function _tokenIds(NestingLevelLib.SignedMoonbirdNestingLevel[] memory sigs)
        internal
        pure
        returns (uint256[] memory)
    {
        uint256[] memory tokenIds = new uint[](sigs.length);
        for (uint256 i; i < sigs.length; ++i) {
            tokenIds[i] = sigs[i].payload.tokenId;
        }
        return tokenIds;
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

    struct TestCase {
        address caller;
        NestingLevelLib.SignedMoonbirdNestingLevel[] sigs;
        uint256 msgValue;
    }

    function _test(TestCase memory tt, bytes memory err)
        internal
        assertUsedTokensMarked(_tokenIds(tt.sigs), err)
        assertNumItemsIncreased(tt.caller, tt.sigs.length, err)
    {
        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            expectSellablePurchaseHandled(tt.msgValue, tt.caller, uint64(tt.sigs.length));
        }
        vm.deal(tt.caller, tt.msgValue);
        vm.prank(tt.caller);
        seller.purchase{value: tt.msgValue}(tt.sigs);
    }

    struct Fuzz {
        address tokenOwner;
        uint64 numTokens;
        uint128[20] tokenIdDeltas;
    }

    function _happyCase(Fuzz memory fuzz) internal virtual returns (TestCase memory) {
        _assumeNotContract(fuzz.tokenOwner);

        fuzz.numTokens = uint64(bound(fuzz.numTokens, 0, fuzz.tokenIdDeltas.length));
        uint256[] memory tokenIds = new uint256[](fuzz.numTokens);

        if (fuzz.numTokens > 0) {
            tokenIds[0] = fuzz.tokenIdDeltas[0];
        }
        for (uint256 i = 1; i < fuzz.numTokens; ++i) {
            // + 1 to guarantee no duplicates
            tokenIds[i] = tokenIds[i - 1] + fuzz.tokenIdDeltas[i] + 1;
        }
        gatingToken.mint(fuzz.tokenOwner, tokenIds);

        NestingLevelLib.SignedMoonbirdNestingLevel[] memory sigs =
            new NestingLevelLib.SignedMoonbirdNestingLevel[](fuzz.numTokens);
        for (uint256 i; i < fuzz.numTokens; ++i) {
            sigs[i] = _sign(
                signerKey,
                NestingLevelLib.MoonbirdNestingLevel({tokenId: tokenIds[i], nestingLevel: seller.requiredNestingLevel()})
            );
        }

        return TestCase({caller: fuzz.tokenOwner, sigs: sigs, msgValue: seller.cost(fuzz.numTokens)});
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testUseTokenTwiceInDifferentTx(Fuzz memory fuzz) public {
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.sigs.length > 0);
        _test(tt, "");
        _test(
            tt,
            abi.encodeWithSelector(TokenUsageTracker.TokenAlreadyUsedForPurchase.selector, tt.sigs[0].payload.tokenId)
        );
    }

    function testUseTokenTwiceInSameTx(Fuzz memory fuzz) public {
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.sigs.length >= 2);
        tt.sigs[tt.sigs.length - 2] = tt.sigs[tt.sigs.length - 1];

        _test(
            tt,
            abi.encodeWithSelector(
                TokenUsageTracker.TokenAlreadyUsedForPurchase.selector, tt.sigs[tt.sigs.length - 2].payload.tokenId
            )
        );
    }

    function testRevertsOnInsufficientNestingLevel(Fuzz memory fuzz, uint256 idx, uint8 level_) public {
        vm.assume(seller.requiredNestingLevel() != NestingLevelLib.NestingLevel.Unnested);

        // This guarantees that level is less than the required nesting level.
        NestingLevelLib.NestingLevel level = NestingLevelLib.NestingLevel(level_ % uint8(seller.requiredNestingLevel()));

        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.sigs.length > 0);

        idx = idx % tt.sigs.length;
        tt.sigs[idx].payload.nestingLevel = level;
        tt.sigs[idx] = _sign(signerKey, tt.sigs[idx].payload);

        _test(
            tt,
            abi.encodeWithSelector(
                MoonbirdNestingLevelGated.InsufficientNestingLevel.selector,
                tt.sigs[idx].payload,
                seller.requiredNestingLevel()
            )
        );
    }

    function testRevertsForUnapprovedSigners(string memory vandalName, Fuzz memory fuzz, uint256 idx) public {
        (address vandal, uint256 vandalKey) = makeAddrAndKey(vandalName);
        vm.assume(vandal != signer);

        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.sigs.length > 0);

        idx = idx % tt.sigs.length;
        tt.sigs[idx] = _sign(vandalKey, tt.sigs[idx].payload);

        _test(tt, abi.encodeWithSelector(MoonbirdNestingLevelGated.UnauthorisedSigner.selector, tt.sigs[idx], vandal));
    }

    function testChangeSigners(string memory newSignerName, Fuzz memory fuzz) public {
        (address newSigner, uint256 newSignerKey) = makeAddrAndKey(newSignerName);
        vm.assume(newSigner != signer);

        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.sigs.length > 0);

        address[] memory rm = new address[](1);
        rm[0] = signer;
        address[] memory add = new address[](1);
        add[0] = newSigner;
        _changeAllowlistSigners(rm, add);

        _test(tt, abi.encodeWithSelector(MoonbirdNestingLevelGated.UnauthorisedSigner.selector, tt.sigs[0], signer));

        for (uint256 i; i < tt.sigs.length; ++i) {
            tt.sigs[i] = _sign(newSignerKey, tt.sigs[i].payload);
        }
        _test(tt, "");
    }

    struct DifferentCallerFuzzParams {
        Fuzz purchase;
        address caller;
    }

    function _differentCallerCase(DifferentCallerFuzzParams memory fuzz) internal returns (TestCase memory) {
        // Excluding zero address because it is "approved" by default
        vm.assume(fuzz.caller != address(0));
        vm.assume(fuzz.caller != fuzz.purchase.tokenOwner);

        TestCase memory tt = _happyCase(fuzz.purchase);
        tt.caller = fuzz.caller;

        return tt;
    }

    function testNotAllowedToClaim(DifferentCallerFuzzParams memory fuzz, uint256 idx) public {
        TestCase memory tt = _differentCallerCase(fuzz);
        vm.assume(tt.sigs.length > 0);

        idx = idx % tt.sigs.length;
        for (uint256 i; i < tt.sigs.length; ++i) {
            if (i == idx) {
                continue;
            }

            vm.prank(fuzz.purchase.tokenOwner);
            gatingToken.approve(tt.caller, tt.sigs[i].payload.tokenId);
        }

        _test(
            tt,
            abi.encodeWithSelector(
                TokenUsageTracker.OperatorNotAllowedToPurchaseWithToken.selector,
                fuzz.caller,
                tt.sigs[idx].payload.tokenId
            )
        );
    }

    function testApproved(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        for (uint256 i; i < tt.sigs.length; ++i) {
            vm.prank(fuzz.purchase.tokenOwner);
            gatingToken.approve(tt.caller, tt.sigs[i].payload.tokenId);
        }

        _test(tt, "");
    }

    function testApprovedForAll(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        vm.prank(fuzz.purchase.tokenOwner);
        gatingToken.setApprovalForAll(tt.caller, true);

        _test(tt, "");
    }
}

abstract contract AMoonbirdNestingLevelGatedPurchaseWithDelegationTest is AMoonbirdNestingLevelGatedPurchaseTest {
    function testDelegatedForAllPurchase(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        vm.prank(fuzz.purchase.tokenOwner);
        delegationRegistry.delegateForAll(tt.caller, true);

        _test(tt, "");
    }

    function testDelegatedContractPurchase(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        vm.prank(fuzz.purchase.tokenOwner);
        delegationRegistry.delegateForContract(tt.caller, address(gatingToken), true);

        _test(tt, "");
    }

    function testDelegatedTokenPurchase(DifferentCallerFuzzParams memory fuzz) public {
        TestCase memory tt = _differentCallerCase(fuzz);

        for (uint256 i; i < tt.sigs.length; ++i) {
            vm.prank(fuzz.purchase.tokenOwner);
            delegationRegistry.delegateForToken(tt.caller, address(gatingToken), tt.sigs[i].payload.tokenId, true);
        }

        _test(tt, "");
    }
}
