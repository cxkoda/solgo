// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {SellerTest} from "./SellerTest.sol";

import {RoleGatedFreeOfCharge} from "proof/sellers/presets/RoleGatedFreeOfCharge.sol";
import {SupplyLimited} from "proof/sellers/base/SupplyLimited.sol";

contract RoleGatedFreeOfChargeTest is SellerTest {
    RoleGatedFreeOfCharge public seller;

    uint64 public constant DEFAULT_PURCHASE_LIMIT = type(uint64).max;

    function setUp() public virtual {
        seller = new RoleGatedFreeOfCharge(admin, steerer, sellable, DEFAULT_PURCHASE_LIMIT);
    }
}

contract VandalTest is RoleGatedFreeOfChargeTest {
    function testCannotPurchase(address vandal, RoleGatedFreeOfCharge.Receiver[] memory receivers) public {
        expectRevertNotSteererThenPrank(vandal);
        seller.purchase(receivers);
    }

    function testCannotPurchaseWithGuardRails(
        address vandal,
        RoleGatedFreeOfCharge.Receiver[] memory receivers,
        uint256 num
    ) public {
        expectRevertNotSteererThenPrank(vandal);
        seller.purchaseWithGuardRails(receivers, num);
    }
}

contract PurchaseTest is RoleGatedFreeOfChargeTest {
    struct TestCase {
        address caller;
        RoleGatedFreeOfCharge.Receiver[] receivers;
    }

    function _toExpectedDeltas(RoleGatedFreeOfCharge.Receiver[] memory receivers)
        internal
        pure
        returns (ExpectedDelta[] memory)
    {
        ExpectedDelta[] memory changes = new ExpectedDelta[](receivers.length);
        for (uint256 i; i < receivers.length; i++) {
            changes[i] = ExpectedDelta({to: receivers[i].to, delta: receivers[i].num});
        }
        return changes;
    }

    function _test(TestCase memory tt, bytes memory err)
        internal
        virtual
        assertMultipleNumItemsIncreased(_toExpectedDeltas(tt.receivers), err)
    {
        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            for (uint256 i; i < tt.receivers.length; i++) {
                expectSellablePurchaseHandled(0, tt.receivers[i].to, tt.receivers[i].num);
            }
        }

        vm.prank(tt.caller);
        seller.purchase(tt.receivers);
    }

    struct Fuzz {
        uint256 numReceiver;
        RoleGatedFreeOfCharge.Receiver[20] receivers;
    }

    function _happyCase(Fuzz memory fuzz) internal view returns (TestCase memory tt) {
        fuzz.numReceiver = bound(fuzz.numReceiver, 0, fuzz.receivers.length);

        tt = TestCase({caller: steerer, receivers: new RoleGatedFreeOfCharge.Receiver[](fuzz.numReceiver)});
        for (uint256 i; i < fuzz.numReceiver; i++) {
            tt.receivers[i] = fuzz.receivers[i];

            // Avoid rejecting too many fuzz runs.
            tt.receivers[i].num = uint64(bound(tt.receivers[i].num, 0, type(uint32).max));
        }

        return tt;
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testAfterAddingSteerer(Fuzz memory fuzz, address newSteerer) public {
        vm.assume(newSteerer != steerer);
        TestCase memory tt = _happyCase(fuzz);
        tt.caller = newSteerer;

        _test(tt, missingRoleError(newSteerer, seller.DEFAULT_STEERING_ROLE()));

        vm.startPrank(admin);
        seller.grantRole(seller.DEFAULT_STEERING_ROLE(), newSteerer);
        vm.stopPrank();

        _test(tt, "");
    }

    function testExceedsPurchaseLimit(RoleGatedFreeOfCharge.Receiver[2] memory receivers, uint64 purchaseLimit)
        public
    {
        vm.assume(uint256(receivers[0].num) + uint256(receivers[1].num) > purchaseLimit);
        seller = new RoleGatedFreeOfCharge(admin, steerer, sellable, purchaseLimit);

        TestCase memory tt = TestCase({caller: steerer, receivers: new RoleGatedFreeOfCharge.Receiver[](2)});
        for (uint256 i; i < receivers.length; i++) {
            tt.receivers[i] = receivers[i];
        }

        _test(
            tt,
            abi.encodeWithSelector(
                SupplyLimited.SupplyLimitExceeded.selector,
                receivers[0].num > purchaseLimit ? receivers[0].num : receivers[1].num,
                purchaseLimit - (receivers[0].num <= purchaseLimit ? receivers[0].num : 0)
            )
        );
    }
}

contract PurchaseWithGuardRailsTest is PurchaseTest {
    function _test(TestCase memory tt, bytes memory err)
        internal
        virtual
        override
        assertMultipleNumItemsIncreased(_toExpectedDeltas(tt.receivers), err)
    {
        uint256 expectedNumSoldAfter = seller.numSold();
        for (uint256 i; i < tt.receivers.length; i++) {
            expectedNumSoldAfter += tt.receivers[i].num;
        }

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            for (uint256 i; i < tt.receivers.length; i++) {
                expectSellablePurchaseHandled(0, tt.receivers[i].to, tt.receivers[i].num);
            }
        }

        vm.prank(tt.caller);
        seller.purchaseWithGuardRails(tt.receivers, expectedNumSoldAfter);
    }

    function testWrongNumSoldAfter(Fuzz memory fuzz, uint256 expectedNumSoldAfter) public {
        TestCase memory tt = _happyCase(fuzz);
        uint256 actualNumSoldAfter = seller.numSold();
        for (uint256 i; i < tt.receivers.length; i++) {
            actualNumSoldAfter += tt.receivers[i].num;
        }
        vm.assume(expectedNumSoldAfter != actualNumSoldAfter);

        vm.expectRevert(
            abi.encodeWithSelector(
                RoleGatedFreeOfCharge.WrongNumSoldAfterPurchase.selector, actualNumSoldAfter, expectedNumSoldAfter
            )
        );

        vm.prank(tt.caller);
        seller.purchaseWithGuardRails(tt.receivers, expectedNumSoldAfter);
    }
}
