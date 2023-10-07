// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "proof/constants/Testing.sol";
import {Create2} from "openzeppelin-contracts/utils/Create2.sol";
import {Address} from "openzeppelin-contracts/utils/Address.sol";

import {
    ISingleRedeemer, BasicSingleRedeemer, BasicSingleRedeemerEvents
} from "proof/redemption/BasicSingleRedeemer.sol";

import {RedeemableTokenStub} from "./BaseRedeemableToken.t.sol";

contract BasicSingleRedeemerTest is ProofTest, BasicSingleRedeemerEvents {
    using Address for address;

    BasicSingleRedeemer public redeemer;

    function setUp() public virtual {
        redeemer = new BasicSingleRedeemer();
    }

    struct TestCase {
        address sender;
        RedeemableTokenStub voucher;
        uint256 voucherId;
    }

    function _test(TestCase memory tt) internal virtual {
        vm.assume(tt.sender != address(0));
        vm.startPrank(steerer);
        tt.voucher.grantRole(tt.voucher.REDEEMER_ROLE(), address(redeemer));
        vm.stopPrank();
        tt.voucher.setSenderApproval(tt.sender, tt.voucherId, true);

        assertFalse(tt.voucher.isRedeemed(tt.voucherId), "voucher already redeemed");

        vm.expectEmit(true, true, true, true, address(redeemer));
        emit VoucherRedeemed(tt.sender, tt.voucher, tt.voucherId);
        vm.prank(tt.sender);
        redeemer.redeem(tt.voucher, tt.voucherId);

        assertTrue(tt.voucher.isRedeemed(tt.voucherId));
    }

    function testSequential(address sender) public {
        RedeemableTokenStub voucher = new RedeemableTokenStub(admin, steerer);
        for (uint256 i; i < 20; ++i) {
            _test(TestCase({sender: sender, voucher: voucher, voucherId: i}));
        }
    }

    struct FuzzParams {
        address sender;
        bytes32 salt;
    }

    function testFuzzed(FuzzParams[] memory fuzz) public {
        vm.assume(fuzz.length < 20);
        for (uint256 i; i < fuzz.length; ++i) {
            address voucherAddress = Create2.computeAddress(
                fuzz[i].salt,
                keccak256(abi.encodePacked(type(RedeemableTokenStub).creationCode, abi.encode(admin, steerer)))
            );

            RedeemableTokenStub voucher = voucherAddress.isContract()
                ? RedeemableTokenStub(voucherAddress)
                : new RedeemableTokenStub{salt: fuzz[i].salt}(admin, steerer);

            _test(
                TestCase({
                    sender: fuzz[i].sender,
                    voucher: voucher,
                    // effectvely unique + random voucherId
                    voucherId: uint256(keccak256(abi.encode(fuzz[i], i)))
                })
            );
        }
    }
}
