// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {SellerTest} from "./SellerTest.sol";

import {ExactInternallyPriced} from "proof/sellers/base/InternallyPriced.sol";
import {PerAddressLimited} from "proof/sellers/base/PerAddressLimited.sol";
import {PublicLimitedExactFixedPrice} from "proof/sellers/presets/PublicLimitedExactFixedPrice.sol";

contract PublicExactFixedPriceTest is SellerTest {
    uint64 public constant DEFAULT_MAX_PER_ADDRESS = 4242;

    PublicLimitedExactFixedPrice public seller;

    function setUp() public virtual {
        seller = new PublicLimitedExactFixedPrice(admin, steerer, sellable, DEFAULT_PRICE, DEFAULT_MAX_PER_ADDRESS);
    }
}

contract VandalTest is PublicExactFixedPriceTest {
    function _expectRevertNotSteerer(address vandal) internal {
        vm.assume(vandal != steerer);
        vm.expectRevert(missingRoleError(vandal, seller.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
    }

    function testCannotSetPrice(address vandal, uint256 newPrice) public {
        _expectRevertNotSteerer(vandal);
        seller.setPrice(newPrice);
    }

    function testCannotSetMaxPerAddress(address vandal, uint64 newMaxPerAddress) public {
        _expectRevertNotSteerer(vandal);
        seller.setMaxPerAddress(newMaxPerAddress);
    }
}

contract SetterTest is PublicExactFixedPriceTest {
    function testSetPrice(uint256 newPrice) public {
        vm.prank(steerer);
        seller.setPrice(newPrice);
        assertEq(seller.cost(1), newPrice);
    }

    function testSetMaxPerAddress(uint64 newMaxPerAddress) public {
        vm.prank(steerer);
        seller.setMaxPerAddress(newMaxPerAddress);
        assertEq(seller.maxPerAddress(), newMaxPerAddress);
    }
}

contract PurchaseTest is PublicExactFixedPriceTest {
    struct TestCase {
        address caller;
        address to;
        uint64 num;
        uint256 value;
    }

    modifier assertNumPurchasedByIncreasedBy(address buyer, uint64 delta, bytes memory err) {
        uint64 numPurchasedBefore = seller.numPurchasedBy(buyer);
        _;
        assertEq(seller.numPurchasedBy(buyer), numPurchasedBefore + (err.length > 0 ? 0 : delta));
    }

    function _test(TestCase memory tt, bytes memory err)
        internal
        assertNumItemsIncreased(tt.to, tt.num, err)
        assertNumPurchasedByIncreasedBy(tt.caller, tt.num, err)
    {
        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            expectSellablePurchaseHandled(tt.value, tt.to, tt.num);
        }

        vm.deal(tt.caller, tt.value);
        vm.prank(tt.caller);
        seller.purchase{value: tt.value}(tt.to, tt.num);
    }

    struct Fuzz {
        address caller;
        address to;
        uint64 num;
    }

    function _happyCase(Fuzz memory fuzz) internal view returns (TestCase memory) {
        fuzz.num = uint64(bound(fuzz.num, 0, seller.maxPerAddress()));
        return TestCase({caller: fuzz.caller, to: fuzz.to, num: fuzz.num, value: seller.cost(fuzz.num)});
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testSetPriceAndPurchase(Fuzz memory fuzz, uint192 newPrice) public {
        vm.prank(steerer);
        seller.setPrice(newPrice);
        assertEq(seller.cost(1), newPrice);

        testHappy(fuzz);
    }

    function testRevertOnWrongPayment(Fuzz memory fuzz, uint256 newPrice) public {
        // avoiding overflows
        newPrice = bound(newPrice, 0, type(uint128).max);
        vm.assume(newPrice != seller.cost(1));

        // the case is generated with the old price, so we expect a revert after changing the price.
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(tt.num > 0);

        vm.prank(steerer);
        seller.setPrice(newPrice);

        _test(tt, abi.encodeWithSelector(ExactInternallyPriced.WrongPayment.selector, tt.value, seller.cost(tt.num)));
    }

    function testRevertOnExhaustingLimit(Fuzz memory fuzz, uint64 numMax, uint64 numFirst) public {
        vm.assume(numMax > 2);
        vm.prank(steerer);
        seller.setMaxPerAddress(numMax);

        numFirst = uint64(bound(numFirst, 1, numMax - 1));
        uint64 numSecond = seller.maxPerAddress() - numFirst;

        fuzz.num = numFirst;
        _test(_happyCase(fuzz), "");

        fuzz.num = numSecond + 1;
        _test(
            _happyCase(fuzz),
            abi.encodeWithSelector(PerAddressLimited.ExceedingMaxPerAddressLimit.selector, numSecond + 1, numSecond)
        );

        fuzz.num = numSecond;
        _test(_happyCase(fuzz), "");

        fuzz.num = 1;
        _test(_happyCase(fuzz), abi.encodeWithSelector(PerAddressLimited.ExceedingMaxPerAddressLimit.selector, 1, 0));
    }

    function testChangeLimitAfterPurchase(Fuzz memory fuzz, uint64 numMaxBefore, uint64 numMaxAfter) public {
        vm.assume(numMaxBefore > numMaxAfter);
        vm.assume(numMaxAfter > 0);
        vm.prank(steerer);
        seller.setMaxPerAddress(numMaxBefore);

        fuzz.num = numMaxBefore;
        _test(_happyCase(fuzz), "");

        vm.prank(steerer);
        seller.setMaxPerAddress(numMaxAfter);

        fuzz.num = 1;
        _test(
            _happyCase(fuzz),
            abi.encodeWithSelector(
                PerAddressLimited.ExceedingMaxPerAddressLimit.selector,
                1,
                int256(uint256(numMaxAfter)) - int256(uint256(numMaxBefore))
            )
        );
    }
}
