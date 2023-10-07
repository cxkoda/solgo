// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {BasicSingleRedeemerTest} from "./BasicSingleRedeemer.t.sol";
import {RedeemableTokenStub} from "./BaseRedeemableToken.t.sol";

import {BasicSingleRecordedRedeemer} from "proof/redemption/BasicSingleRecordedRedeemer.sol";

contract BasicSingleRecordedRedeemerTest is BasicSingleRedeemerTest {
    BasicSingleRecordedRedeemer public recordedRedeemer;

    function setUp() public virtual override {
        recordedRedeemer = new BasicSingleRecordedRedeemer();
        redeemer = recordedRedeemer;
    }

    /**
     * @notice Reusing the tests for `BasicSingleRedeemerTest` here.
     * The contract should behave exactly the same + additional bookkeeping
     */
    function _test(TestCase memory tt) internal virtual override {
        uint256 numRedeemedBefore = recordedRedeemer.numVouchersRedeemed(tt.sender, tt.voucher);
        uint256[] memory redeemedVoucherIdsBefore = recordedRedeemer.redeemedVoucherIds(tt.sender, tt.voucher);

        super._test(tt);

        assertEq(
            recordedRedeemer.numVouchersRedeemed(tt.sender, tt.voucher),
            numRedeemedBefore + 1,
            "redeemer.numVouchersRedeemed() wrong"
        );
        assertEq(
            recordedRedeemer.redeemedVoucherIdAt(tt.sender, tt.voucher, numRedeemedBefore),
            tt.voucherId,
            "redeemer.redeemedVoucherIdAt() wrong"
        );

        uint256[] memory redeemedVoucherIdsAfter = recordedRedeemer.redeemedVoucherIds(tt.sender, tt.voucher);
        assertEq(
            redeemedVoucherIdsAfter.length, numRedeemedBefore + 1, "redeemer.redeemedVoucherIds() has wrong length"
        );
        for (uint256 i; i < numRedeemedBefore; ++i) {
            assertEq(
                redeemedVoucherIdsAfter[i],
                redeemedVoucherIdsBefore[i],
                "Previous values of redeemer.redeemedVoucherIds() have changed"
            );
        }
        assertEq(
            redeemedVoucherIdsAfter[numRedeemedBefore], tt.voucherId, "Wrong id pushed to redeemer.redeemedVoucherIds()"
        );
    }
}
