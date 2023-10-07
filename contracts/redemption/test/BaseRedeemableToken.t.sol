// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "proof/constants/Testing.sol";

import {IRedeemableToken} from "../src/interfaces/IRedeemableToken.sol";

import {RedeemableTokenStub} from "./RedeemableTokenStub.t.sol";

contract RedeemableTokenTest is ProofTest {
    RedeemableTokenStub public voucher;

    bytes32 public immutable REDEEMER_ROLE;

    constructor() {
        REDEEMER_ROLE = (new RedeemableTokenStub(address(0), address(0))).REDEEMER_ROLE();
    }

    function setUp() public {
        voucher = new RedeemableTokenStub(admin, steerer);
    }

    function setRedeemerApproval(address redeemer, bool toggle) public {
        vm.startPrank(steerer);
        if (toggle) {
            voucher.grantRole(REDEEMER_ROLE, redeemer);
        } else {
            voucher.revokeRole(REDEEMER_ROLE, redeemer);
        }
        vm.stopPrank();
    }
}

contract VandalTest is RedeemableTokenTest {
    function testCannotLockRedeemers(address vandal) public {
        expectRevertNotSteererThenPrank(vandal);
        voucher.lockRedeemers();
    }
}

contract RedeemTest is RedeemableTokenTest {
    struct TestCase {
        address redeemer;
        bool redeemerApproved;
        address sender;
        bool senderApproved;
        uint256 voucherId;
        bytes err;
    }

    function _test(TestCase memory tt) internal {
        assertEq(voucher.isRedeemed(tt.voucherId), false);
        setRedeemerApproval(tt.redeemer, tt.redeemerApproved);
        voucher.setSenderApproval(tt.sender, tt.voucherId, tt.senderApproved);

        bool fails = tt.err.length > 0;
        if (fails) {
            vm.expectRevert(tt.err);
        }

        vm.prank(tt.redeemer);
        voucher.redeem(tt.sender, tt.voucherId);
        assertEq(voucher.isRedeemed(tt.voucherId), !fails);
    }

    function testRedeem(address redeemer, address sender, uint8 voucherId) public {
        _test(
            TestCase({
                redeemer: redeemer,
                redeemerApproved: true,
                sender: sender,
                senderApproved: true,
                voucherId: voucherId,
                err: hex""
            })
        );
    }

    function testRedeemerNotApproved(address redeemer, address sender, uint8 voucherId) public {
        _test(
            TestCase({
                redeemer: redeemer,
                redeemerApproved: false,
                sender: sender,
                senderApproved: true,
                voucherId: voucherId,
                err: missingRoleError(redeemer, voucher.REDEEMER_ROLE())
            })
        );
    }

    function testSenderNotApproved(address redeemer, address sender, uint8 voucherId) public {
        _test(
            TestCase({
                redeemer: redeemer,
                redeemerApproved: true,
                sender: sender,
                senderApproved: false,
                voucherId: voucherId,
                err: abi.encodeWithSelector(
                    IRedeemableToken.RedeemerCallerNotAllowedToSpendVoucher.selector, sender, voucherId
                    )
            })
        );
    }
}

contract SetRedeemerApprovalTest is RedeemableTokenTest {
    struct TestCase {
        address redeemer;
        bool redeemerApproved;
    }

    function _test(TestCase memory tt, bytes memory err) internal {
        bool isRedeemerBefore = voucher.hasRole(voucher.REDEEMER_ROLE(), tt.redeemer);

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        }
        setRedeemerApproval(tt.redeemer, tt.redeemerApproved);

        bool isRedeemerAfter = voucher.hasRole(voucher.REDEEMER_ROLE(), tt.redeemer);
        assertEq(isRedeemerAfter, fails ? isRedeemerBefore : tt.redeemerApproved);
    }

    function testHappyRepeated(address redeemer, bool[10] memory approval) public {
        for (uint256 i = 0; i < approval.length; i++) {
            _test(TestCase({redeemer: redeemer, redeemerApproved: approval[i]}), hex"");
        }
    }

    function testLockRedeemers(TestCase memory tt) public {
        _test(tt, "");

        vm.prank(steerer);
        voucher.lockRedeemers();

        _test(tt, missingRoleError(steerer, keccak256("NOOP_ROLE")));
    }
}
