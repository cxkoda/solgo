// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {SellerTest, ProofTest} from "./SellerTest.sol";

import {Seller, SignatureGated, SignatureGatedLib} from "proof/sellers/mechanics/SignatureGated.sol";
import {PurchaseExecuter} from "proof/sellers/interfaces/PurchaseExecuter.sol";

import {ISellable, ImmutableSellableCallbacker, SellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

contract SignatureGatedFake is SignatureGated, ImmutableSellableCallbacker, ProofTest {
    bytes32 internal _digest;
    SignatureGatedLib.SignedAllowance internal _signedAllowance;

    constructor(ISellable s) ImmutableSellableCallbacker(s) {}

    function _purchaseWithSignedAllowance(SignedAllowancePurchase calldata purchase_) internal virtual override {
        // Instrumenting this function to ensure we are correctly recovering this data in `_executePurchase`.
        _digest = digest(purchase_.signedAllowance.allowance);
        _signedAllowance = purchase_.signedAllowance;
        super._purchaseWithSignedAllowance(purchase_);
    }

    function _executePurchase(address to, uint64 num, uint256 cost, bytes memory data)
        internal
        virtual
        override(PurchaseExecuter, SellableCallbacker)
    {
        (bytes32 d, SignatureGatedLib.SignedAllowance memory sa) = SignatureGatedLib.decodePurchaseData(data);
        assertEq(d, _digest);
        assertEq(keccak256(abi.encode(sa)), keccak256(abi.encode(_signedAllowance)));

        SellableCallbacker._executePurchase(to, num, cost, data);
    }

    function changeAllowlistSigners(address[] calldata rm, address[] calldata add) public {
        _changeAllowlistSigners(rm, add);
    }
}

abstract contract ASignatureGatedTest is SellerTest {
    SignatureGated public seller;

    address public signer;
    uint256 public signerKey;

    constructor() {
        (signer, signerKey) = makeAddrAndKey("signer");
    }

    function setUp() public virtual;

    function _sign(uint256 key, SignatureGatedLib.Allowance memory allowance)
        internal
        view
        returns (SignatureGatedLib.SignedAllowance memory)
    {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(key, seller.digest(allowance));
        return SignatureGatedLib.SignedAllowance({allowance: allowance, signature: abi.encodePacked(r, s, v)});
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual;
}

abstract contract APurchaseTest is ASignatureGatedTest {
    struct TestCase {
        address caller;
        SignatureGated.SignedAllowancePurchase purchase;
        uint256 value;
    }

    function _toAllowances(SignatureGatedLib.Allowance[1] memory input)
        internal
        pure
        returns (SignatureGatedLib.Allowance[] memory output)
    {
        output = new  SignatureGatedLib.Allowance[](input.length);
        output[0] = input[0];
    }

    modifier assertNumClaimedWithAllowanceIncreased(
        SignatureGatedLib.Allowance memory allowance,
        uint256 delta,
        bytes memory err
    ) {
        uint256 numBefore = seller.numPurchasedWithAllowances(_toAllowances([allowance]))[0];
        _;
        bool fails = err.length > 0;
        assertEq(
            seller.numPurchasedWithAllowances(_toAllowances([allowance]))[0],
            numBefore + (fails ? 0 : delta),
            "seller.numPurchasedWithAllowance(allowance) not increased correctly"
        );
    }

    function _test(TestCase memory tt, bytes memory err)
        internal
        assertNumItemsIncreased(tt.purchase.signedAllowance.allowance.receiver, tt.purchase.num, err)
        assertNumClaimedWithAllowanceIncreased(tt.purchase.signedAllowance.allowance, tt.purchase.num, err)
    {
        vm.assume(tt.purchase.num > 0);

        SignatureGated.SignedAllowancePurchase[] memory purchases = new SignatureGated.SignedAllowancePurchase[](1);
        purchases[0] = tt.purchase;

        if (err.length > 0) {
            vm.expectRevert(err);
        } else {
            // We have to store data in a seperate variable here and cannot inline it below because of stack-too-deep
            // errors.
            bytes memory data = SignatureGatedLib.encodePurchaseData(
                seller.digest(tt.purchase.signedAllowance.allowance), tt.purchase.signedAllowance
            );
            vm.expectEmit(true, true, true, true, address(sellable));
            emit SellablePurchaseHandled(
                address(seller),
                tt.purchase.num * tt.purchase.signedAllowance.allowance.price,
                tt.purchase.signedAllowance.allowance.receiver,
                tt.purchase.num,
                data
            );
        }
        vm.deal(tt.caller, tt.value);
        vm.prank(tt.caller);
        seller.purchase{value: tt.value}(purchases);
    }

    struct Fuzz {
        address caller;
        address receiver;
        uint64 numMax;
        uint256 nonce;
        uint128 price;
    }

    function _allowance(Fuzz memory fuzz) internal pure returns (SignatureGatedLib.Allowance memory) {
        return SignatureGatedLib.Allowance({
            receiver: fuzz.receiver,
            nonce: fuzz.nonce,
            numMax: fuzz.numMax,
            price: fuzz.price,
            activeAfterTimestamp: 0,
            activeUntilTimestamp: type(uint256).max
        });
    }

    function _happyCase(Fuzz memory fuzz, SignatureGatedLib.Allowance memory allowance)
        internal
        view
        returns (TestCase memory)
    {
        return TestCase({
            caller: fuzz.caller,
            purchase: SignatureGated.SignedAllowancePurchase({
                signedAllowance: _sign(signerKey, allowance),
                num: allowance.numMax
            }),
            value: uint256(allowance.price) * allowance.numMax
        });
    }

    function _happyCase(Fuzz memory fuzz) internal view returns (TestCase memory) {
        return _happyCase(fuzz, _allowance(fuzz));
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testHappyIteratively(Fuzz memory fuzz) public {
        vm.assume(fuzz.numMax < 20); // Speeding up tests

        TestCase memory tt = _happyCase(fuzz);
        tt.purchase.num = 1;
        tt.value = fuzz.price;

        for (uint256 i; i < fuzz.numMax; ++i) {
            _test(tt, "");
        }
        // We are exhausting the allowance in the above loop, so purchasing one more must fail.
        _test(
            tt,
            abi.encodeWithSelector(
                SignatureGated.TooManyPurchasesRequested.selector, tt.purchase.signedAllowance.allowance, 0, 1
            )
        );
    }

    function testHappyWithDifferentNonce(Fuzz memory fuzz, uint256 differentNonce) public {
        vm.assume(fuzz.nonce != differentNonce);
        _test(_happyCase(fuzz), "");

        fuzz.nonce = differentNonce;
        _test(_happyCase(fuzz), "");
    }

    function testCannotClaimTooMany(Fuzz memory fuzz, uint64 numFirst) public {
        vm.assume(numFirst < fuzz.numMax);
        TestCase memory tt = _happyCase(fuzz);

        tt.purchase.num = numFirst;
        tt.value = tt.purchase.num * tt.purchase.signedAllowance.allowance.price;
        _test(tt, "");

        tt.purchase.num = fuzz.numMax - numFirst + 1;
        tt.value = tt.purchase.num * tt.purchase.signedAllowance.allowance.price;
        _test(
            tt,
            abi.encodeWithSelector(
                SignatureGated.TooManyPurchasesRequested.selector,
                tt.purchase.signedAllowance.allowance,
                fuzz.numMax - numFirst,
                tt.purchase.num
            )
        );
    }

    function testUnderpayment(Fuzz memory fuzz, uint256 reduction) public virtual {
        vm.assume(fuzz.price > 0);
        vm.assume(reduction > 0);
        TestCase memory tt = _happyCase(fuzz);
        vm.assume(reduction < tt.value);

        tt.value -= reduction;
        // This fails because msg.value is not sufficient to do the cost forwarding on the sellable callbacks.
        // Unfortunately reason the returned reason string is empty.
        _test(tt, abi.encodeWithSelector(SellableCallbacker.CallbackFailed.selector, ""));
    }

    function testOverpayment(Fuzz memory fuzz, uint64 increase) public virtual {
        vm.assume(fuzz.price > 0);
        vm.assume(increase > 0);
        TestCase memory tt = _happyCase(fuzz);

        tt.value += increase;
        _test(tt, "");
    }

    function testMustRejectUnapprovedSigners(string memory vandalName, Fuzz memory fuzz) public {
        (address vandal, uint256 vandalKey) = makeAddrAndKey(vandalName);
        vm.assume(vandal != signer);

        TestCase memory tt = _happyCase(fuzz);
        tt.purchase.signedAllowance = _sign(vandalKey, tt.purchase.signedAllowance.allowance);

        _test(
            tt, abi.encodeWithSelector(SignatureGated.UnauthorisedSigner.selector, tt.purchase.signedAllowance, vandal)
        );
    }

    function testChangeSigners(string memory newSignerName, Fuzz memory fuzz) public {
        (address newSigner, uint256 newSignerKey) = makeAddrAndKey(newSignerName);
        vm.assume(newSigner != signer);

        address[] memory rm = new address[](1);
        rm[0] = signer;
        address[] memory add = new address[](1);
        add[0] = newSigner;
        _changeAllowlistSigners(rm, add);

        TestCase memory tt = _happyCase(fuzz);
        _test(
            tt, abi.encodeWithSelector(SignatureGated.UnauthorisedSigner.selector, tt.purchase.signedAllowance, signer)
        );

        tt.purchase.signedAllowance = _sign(newSignerKey, tt.purchase.signedAllowance.allowance);
        _test(tt, "");
    }

    function testCannotUseExpiredSignature(Fuzz memory fuzz, uint128 activeUntilTimestamp) public {
        vm.assume(activeUntilTimestamp > 0);

        SignatureGatedLib.Allowance memory allowance = _allowance(fuzz);
        allowance.activeUntilTimestamp = activeUntilTimestamp;

        vm.warp(activeUntilTimestamp);
        _test(_happyCase(fuzz, allowance), abi.encodeWithSelector(SignatureGated.InactiveAllowance.selector, allowance));

        vm.warp(activeUntilTimestamp - 1);
        _test(_happyCase(fuzz, allowance), "");
    }

    function testCannotUseInactiveSignature(Fuzz memory fuzz, uint128 activeAfterTimestamp) public {
        vm.assume(activeAfterTimestamp > 0);

        SignatureGatedLib.Allowance memory allowance = _allowance(fuzz);
        allowance.activeAfterTimestamp = activeAfterTimestamp;

        vm.warp(activeAfterTimestamp - 1);
        _test(_happyCase(fuzz, allowance), abi.encodeWithSelector(SignatureGated.InactiveAllowance.selector, allowance));

        vm.warp(activeAfterTimestamp);
        _test(_happyCase(fuzz, allowance), "");
    }

    function _signAndPurchaseMultiple(address buyer, SignatureGatedLib.Allowance[3] memory allowances, bytes memory err)
        internal
    {
        SignatureGated.SignedAllowancePurchase[] memory purchases =
            new SignatureGated.SignedAllowancePurchase[](allowances.length);
        uint256 totalValue = 0;
        for (uint256 i = 0; i < allowances.length; i++) {
            purchases[i] = SignatureGated.SignedAllowancePurchase({
                signedAllowance: _sign(signerKey, allowances[i]),
                num: allowances[i].numMax
            });
            totalValue += allowances[i].numMax * allowances[i].price;
        }

        if (err.length > 0) {
            vm.expectRevert(err);
        }

        vm.deal(buyer, totalValue);
        vm.prank(buyer);
        seller.purchase{value: totalValue}(purchases);
    }

    function testHappyMultiple(address alice, address bob, address buyer) public {
        vm.assume(alice != bob);
        vm.assume(alice != buyer);
        vm.assume(bob != buyer);
        SignatureGatedLib.Allowance[3] memory allowances = [
            SignatureGatedLib.Allowance({
                receiver: alice,
                nonce: 0,
                numMax: 42,
                price: 1,
                activeAfterTimestamp: 0,
                activeUntilTimestamp: type(uint256).max
            }),
            SignatureGatedLib.Allowance({
                receiver: bob,
                nonce: 0,
                numMax: 42,
                price: 2,
                activeAfterTimestamp: 0,
                activeUntilTimestamp: type(uint256).max
            }),
            SignatureGatedLib.Allowance({
                receiver: alice,
                nonce: 0,
                numMax: 1337,
                price: 3,
                activeAfterTimestamp: 0,
                activeUntilTimestamp: type(uint256).max
            })
        ];

        _signAndPurchaseMultiple(buyer, allowances, "");

        assertEq(sellable.numItems(alice), allowances[0].numMax + allowances[2].numMax);
        assertEq(sellable.numItems(bob), allowances[1].numMax);
        assertEq(sellable.numItems(buyer), 0);
    }

    function testMultipleWithOneInactive(address alice, address bob, address buyer) public {
        vm.assume(alice != bob);
        vm.assume(alice != buyer);
        vm.assume(bob != buyer);
        vm.warp(0);
        SignatureGatedLib.Allowance[3] memory allowances = [
            SignatureGatedLib.Allowance({
                receiver: alice,
                nonce: 0,
                numMax: 42,
                price: 1,
                activeAfterTimestamp: 0,
                activeUntilTimestamp: type(uint256).max
            }),
            SignatureGatedLib.Allowance({
                receiver: bob,
                nonce: 0,
                numMax: 42,
                price: 2,
                activeAfterTimestamp: 100, // Sneaking in an inactive signature
                activeUntilTimestamp: type(uint256).max
            }),
            SignatureGatedLib.Allowance({
                receiver: alice,
                nonce: 0,
                numMax: 1337,
                price: 3,
                activeAfterTimestamp: 0,
                activeUntilTimestamp: type(uint256).max
            })
        ];

        _signAndPurchaseMultiple(
            buyer, allowances, abi.encodeWithSelector(SignatureGated.InactiveAllowance.selector, allowances[1])
        );
    }
}

contract SignatureGatedTest is ASignatureGatedTest {
    SignatureGatedFake public impl;

    function setUp() public virtual override {
        impl = new SignatureGatedFake(sellable);
        seller = impl;
        _changeAllowlistSigners(new address[](0), toAddresses([signer]));
    }

    function _changeAllowlistSigners(address[] memory rm, address[] memory add) internal virtual override {
        impl.changeAllowlistSigners(rm, add);
    }
}

contract PurchaseTest is APurchaseTest, SignatureGatedTest {}

contract DigestEncodingTest is ProofTest {
    function testEncoding(bytes32 digest, SignatureGatedLib.SignedAllowance calldata signedAllowance) public {
        (bytes32 d, SignatureGatedLib.SignedAllowance memory sa) =
            SignatureGatedLib.decodePurchaseData(SignatureGatedLib.encodePurchaseData(digest, signedAllowance));
        assertEq(d, digest);
        assertEq(keccak256(abi.encode(sa)), keccak256(abi.encode(signedAllowance)));
    }
}
