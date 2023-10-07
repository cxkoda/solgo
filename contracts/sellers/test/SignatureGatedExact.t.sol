// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {Test} from "ethier_root/tests/TestLib.sol";
import {console2} from "forge-std/console2.sol";

import {
    CallbackerWithAccessControl,
    SignatureGated,
    SignatureGatedExact
} from "proof/sellers/presets/SignatureGatedExact.sol";
import {ASignatureGatedTest, APurchaseTest} from "./SignatureGated.t.sol";

contract SignatureGatedExactTest is ASignatureGatedTest {
    SignatureGatedExact public impl;

    function setUp() public virtual override {
        impl = new SignatureGatedExact(admin, steerer, sellable);
        seller = impl;
        _changeAllowlistSigners(new address[](0), toAddresses([signer]));
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual override {
        vm.prank(steerer);
        impl.changeAllowlistSigners(rm, add);
    }
}

contract VandalTest is SignatureGatedExactTest {
    function testCannotChangeSigners(address vandal, address[] memory rm, address[] memory add) public {
        vm.assume(vandal != steerer);
        vm.expectRevert(missingRoleError(vandal, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        impl.changeAllowlistSigners(rm, add);
    }
}

contract PurchaseTest is APurchaseTest, SignatureGatedExactTest {
    function testUnderpayment(Fuzz memory fuzz, uint256 reduction) public virtual override {
        vm.assume(fuzz.price > 0);
        vm.assume(reduction > 0);
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(reduction < tt.value);

        tt.value -= reduction;
        _test(
            tt,
            abi.encodeWithSelector(
                SignatureGatedExact.WrongPayment.selector,
                tt.value,
                tt.purchase.signedAllowance.allowance.price * tt.purchase.num
            )
        );
    }

    function testOverpayment(Fuzz memory fuzz, uint64 increase) public virtual override {
        vm.assume(fuzz.price > 0);
        vm.assume(increase > 0);
        TestCase memory tt = _happyCase(fuzz);

        tt.value += increase;
        _test(
            tt,
            abi.encodeWithSelector(
                SignatureGatedExact.WrongPayment.selector,
                tt.value,
                tt.purchase.signedAllowance.allowance.price * tt.purchase.num
            )
        );
    }
}
