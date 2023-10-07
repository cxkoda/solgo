// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import "forge-std/console2.sol";

import {ProofTest} from "proof/constants/Testing.sol";
import {Address} from "openzeppelin-contracts/utils/Address.sol";

import {IRedeemableToken} from "proof/redemption/interfaces/IRedeemableToken.sol";
import {RankingRedeemer, RankingRedeemerEvents} from "proof/redemption/RankingRedeemer.sol";

import {RedeemableTokenStub} from "./BaseRedeemableToken.t.sol";

contract RankingRedeemerTest is ProofTest, RankingRedeemerEvents {
    using Address for address;

    RedeemableTokenStub public token;

    uint256 public constant SAFE_PRIME = 1367;

    function setUp() public virtual {
        token = new RedeemableTokenStub(admin, steerer);
    }
}

contract RedeemMultipleTest is RankingRedeemerTest {
    struct TestCase {
        RankingRedeemer redeemer;
        address sender;
        bool senderApproval;
        RankingRedeemer.Redemption[] redemptions;
    }

    function _test(TestCase memory tt, bytes memory err) internal virtual {
        vm.assume(tt.sender != address(0));

        for (uint256 i = 0; i < tt.redemptions.length; i++) {
            assertFalse(token.isRedeemed(tt.redemptions[i].tokenId), "voucher already redeemed");
            token.setSenderApproval(tt.sender, tt.redemptions[i].tokenId, tt.senderApproval);
        }

        vm.startPrank(steerer);
        token.grantRole(token.REDEEMER_ROLE(), address(tt.redeemer));
        vm.stopPrank();

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            for (uint256 i = 0; i < tt.redemptions.length; i++) {
                vm.expectEmit(true, true, true, true, address(tt.redeemer));
                emit VoucherRedeemedAndRankingCommited(
                    tt.sender, token, tt.redemptions[i].tokenId, tt.redemptions[i].ranking
                );
            }
        }

        vm.prank(tt.sender);
        tt.redeemer.redeem(tt.redemptions);

        for (uint256 i = 0; i < tt.redemptions.length; i++) {
            assertEq(token.isRedeemed(tt.redemptions[i].tokenId), !fails);
        }
    }

    struct Fuzz {
        address sender;
        uint8 numChoices;
        uint256 numTokens;
        uint32[20] tokenIdDeltas;
        uint256 seed;
    }

    function _happyCase(Fuzz memory fuzz) internal returns (TestCase memory) {
        fuzz.numTokens = bound(fuzz.numTokens, 1, fuzz.tokenIdDeltas.length);
        uint256[] memory tokenIds = deltasToUniqueAbsolute(toUint256s(fuzz.tokenIdDeltas));

        RankingRedeemer.Redemption[] memory redemptions = new RankingRedeemer.Redemption[](fuzz.numTokens);
        for (uint256 i = 0; i < fuzz.numTokens; i++) {
            redemptions[i] = RankingRedeemer.Redemption({
                redeemable: token,
                tokenId: tokenIds[i],
                ranking: toUint8s(quickShuffle(sequence(0, fuzz.numChoices), uint256(keccak256(abi.encode(fuzz.seed, i)))))
            });
        }

        return TestCase({
            sender: fuzz.sender,
            senderApproval: true,
            redemptions: redemptions,
            redeemer: new RankingRedeemer(fuzz.numChoices)
        });
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testInvalidLength(Fuzz memory fuzz, uint256 redemptionIdx, uint8 differentNumChoices) public {
        vm.assume(differentNumChoices != fuzz.numChoices);
        TestCase memory tt = _happyCase(fuzz);

        redemptionIdx = redemptionIdx % tt.redemptions.length;
        tt.redemptions[redemptionIdx].ranking = new uint8[](differentNumChoices);

        _test(
            tt,
            abi.encodeWithSelector(
                RankingRedeemer.InvalidRankingLength.selector,
                tt.redemptions[redemptionIdx],
                differentNumChoices,
                fuzz.numChoices
            )
        );
    }

    function testInvalidRanking(Fuzz memory fuzz, uint256 redemptionIdx, uint8 corruptedIdx, uint8 corruptedValue)
        public
    {
        vm.assume(fuzz.numChoices > 0);

        TestCase memory tt = _happyCase(fuzz);
        redemptionIdx = redemptionIdx % tt.redemptions.length;
        corruptedIdx = corruptedIdx % fuzz.numChoices;

        // replaces one choice in a ranking with a random, invalid value.
        // e.g. if our ranking was `[0,1,2,3]` it could be `[0,1,47,3]` after corruption.
        // since choice 2 is missing we revert with InvalidRanking.

        uint8 original = tt.redemptions[redemptionIdx].ranking[corruptedIdx];
        vm.assume(original != corruptedValue);

        uint256 wantRevertBitmask = (1 << fuzz.numChoices) - 1;
        wantRevertBitmask -= (1 << original);
        wantRevertBitmask |= (1 << corruptedValue);

        tt.redemptions[redemptionIdx].ranking[corruptedIdx] = corruptedValue;

        _test(
            tt,
            abi.encodeWithSelector(
                RankingRedeemer.InvalidRanking.selector, tt.redemptions[redemptionIdx], wantRevertBitmask
            )
        );
    }

    function testUnapprovedSender(Fuzz memory fuzz) public {
        TestCase memory tt = _happyCase(fuzz);
        tt.senderApproval = false;
        _test(
            tt,
            abi.encodeWithSelector(
                IRedeemableToken.RedeemerCallerNotAllowedToSpendVoucher.selector, tt.sender, tt.redemptions[0].tokenId
            )
        );
    }
}
