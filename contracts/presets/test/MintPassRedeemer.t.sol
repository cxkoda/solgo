// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.

pragma solidity >=0.8.0 <0.9.0;

import {SellerTest} from "proof/sellers/../test/SellerTest.sol";

import {IRedeemableToken} from "proof/redemption/interfaces/IRedeemableToken.sol";
import {BaseRedeemableToken, RedeemableTokenStub} from "proof/redemption/../test/RedeemableTokenStub.t.sol";

import {PurchaseByProjectIDLib} from "proof/sellers/sellable/SellableERC721ACommonByProjectID.sol";

import {
    MintPassRedeemer,
    MintPassForProjectIDRedeemer,
    FreeMintPassForProjectIDRedeemer
} from "../src/MintPassRedeemer.sol";

// TODO add tests for non-free redeemer

contract FreeMintPassForProjectIDRedeemerTest is SellerTest {
    FreeMintPassForProjectIDRedeemer public redeemer;
    RedeemableTokenStub public pass;

    function setUp() public {
        pass = new RedeemableTokenStub(admin, steerer);
        redeemer = new FreeMintPassForProjectIDRedeemer(sellable, pass);

        vm.startPrank(steerer);
        pass.grantRole(pass.REDEEMER_ROLE(), address(redeemer));
        vm.stopPrank();
    }
}

contract RedeemTest is FreeMintPassForProjectIDRedeemerTest {
    function _redeem(address caller, MintPassForProjectIDRedeemer.Redemption[] memory redemptions, bytes memory err)
        internal
        assertNumItemsIncreased(caller, redemptions.length, err)
    {
        uint128[] memory projectIds = new uint128[](redemptions.length);
        for (uint256 i; i < redemptions.length; ++i) {
            projectIds[i] = redemptions[i].projectId;
        }

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            expectSellablePurchaseHandled({
                seller: address(redeemer),
                value: 0,
                to: caller,
                num: uint64(redemptions.length),
                data: PurchaseByProjectIDLib.encodePurchaseData(projectIds)
            });
        }

        vm.prank(caller);
        redeemer.redeem(redemptions);
    }

    function testRedeem(address owner) public {
        pass.setSenderApproval(owner, 1, true);
        pass.setSenderApproval(owner, 2, true);

        MintPassForProjectIDRedeemer.Redemption[] memory redemptions = new MintPassForProjectIDRedeemer.Redemption[](2);
        redemptions[0] = MintPassForProjectIDRedeemer.Redemption({passId: 1, projectId: 4});
        redemptions[1] = MintPassForProjectIDRedeemer.Redemption({passId: 2, projectId: 3});

        _redeem(owner, redemptions, "");
    }

    function testRedeemFuzzed(address owner, uint256 passId, uint128 projectId) public {
        pass.setSenderApproval(owner, passId, true);

        MintPassForProjectIDRedeemer.Redemption[] memory redemptions = new MintPassForProjectIDRedeemer.Redemption[](1);
        redemptions[0] = MintPassForProjectIDRedeemer.Redemption({passId: passId, projectId: projectId});

        _redeem(owner, redemptions, "");
    }

    function testNotOwner(address owner, address caller, uint256 passId) public {
        vm.assume(caller != owner);
        pass.setSenderApproval(owner, passId, true);
        pass.setSenderApproval(caller, passId, false);

        MintPassForProjectIDRedeemer.Redemption[] memory redemptions = new MintPassForProjectIDRedeemer.Redemption[](1);
        redemptions[0] = MintPassForProjectIDRedeemer.Redemption({passId: passId, projectId: 0});

        _redeem(
            caller,
            redemptions,
            abi.encodeWithSelector(
                MintPassRedeemer.RedeemableCallbackFailed.selector,
                pass,
                passId,
                abi.encodeWithSelector(IRedeemableToken.RedeemerCallerNotAllowedToSpendVoucher.selector, caller, passId)
            )
        );
    }

    function testCannotRedeemTwice(address owner, uint256 passId, uint128 projectId) public {
        pass.setSenderApproval(owner, passId, true);

        MintPassForProjectIDRedeemer.Redemption[] memory redemptions = new MintPassForProjectIDRedeemer.Redemption[](2);
        redemptions[0] = MintPassForProjectIDRedeemer.Redemption({passId: passId, projectId: projectId});
        redemptions[1] = MintPassForProjectIDRedeemer.Redemption({passId: passId, projectId: projectId});

        _redeem(
            owner,
            redemptions,
            abi.encodeWithSelector(
                MintPassRedeemer.RedeemableCallbackFailed.selector,
                pass,
                passId,
                abi.encodeWithSelector(RedeemableTokenStub.AlreadyRedeemed.selector, passId)
            )
        );
    }
}
