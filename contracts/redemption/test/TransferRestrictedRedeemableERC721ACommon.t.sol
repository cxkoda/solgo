// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "proof/constants/Testing.sol";
import {IERC721A} from "ERC721A/interfaces/IERC721A.sol";

import {IRedeemableToken} from "../src/interfaces/IRedeemableToken.sol";
import {BasicSingleRecordedRedeemer} from "../src/BasicSingleRecordedRedeemer.sol";
import {ERC721ACommon, RedeemableERC721ACommon} from "../src/voucher/RedeemableERC721ACommon.sol";

import {
    TransferRestrictedRedeemableERC721ACommon,
    TransferRestriction
} from "../src/voucher/TransferRestrictedRedeemableERC721ACommon.sol";

import {RedeemableERC721ACommonFake, RedeemTest} from "./RedeemableERC721ACommon.t.sol";
import {ERC721ATransferRestrictedBase} from "../src/restricted/ERC721ATransferRestrictedBase.sol";

contract TransferRestrictedRedeemableERC721ACommonFake is TransferRestrictedRedeemableERC721ACommon {
    constructor(address admin, address steerer)
        ERC721ACommon(admin, steerer, "", "", payable(address(0x000FEE00)), 750)
    {}

    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }
}

/**
 * @dev This runs the same tests as RedeemTest on the new redeemable contract with transfer restrictions available but
 *  none enabled.
 */
contract TransferRestrictedRedeemableERC721ACommonFake_NoRestriction_RedeemTest is RedeemTest {
    function setUp() public virtual override {
        voucher =
            RedeemableERC721ACommonFake(address(new TransferRestrictedRedeemableERC721ACommonFake(admin, steerer)));
        redeemer = new BasicSingleRecordedRedeemer();
    }
}

/**
 * @dev This runs the same tests as RedeemTest on the new redeemable contract with OnlyBurn transfer restriction enabled.
 * This should not affect any of the tests since they are only redeeming, which results in tokens being burnt by the
 * redemption.
 */
contract TransferRestrictedRedeemableERC721ACommonFake_OnlyBurn_RedeemTest is RedeemTest {
    TransferRestrictedRedeemableERC721ACommonFake impl;

    function setUp() public virtual override {
        impl = new TransferRestrictedRedeemableERC721ACommonFake(admin, steerer);
        voucher = RedeemableERC721ACommonFake(address(impl));
        redeemer = new BasicSingleRecordedRedeemer();
    }

    function _beforeSetup() internal virtual override {
        vm.prank(steerer);
        impl.setTransferRestriction(TransferRestriction.None);
    }

    function _afterSetup() internal virtual override {
        vm.prank(steerer);
        impl.setTransferRestriction(TransferRestriction.OnlyBurn);
    }
}

/**
 * @dev Same as above but with OnlyMint transfer restriction enabled. All tests must fail since burning is now disabled.
 */
abstract contract TransferRestrictedRedeemableERC721ACommonFake_RedeemTestWithRestriction is RedeemTest {
    TransferRestrictedRedeemableERC721ACommonFake impl;

    function setUp() public virtual override {
        impl = new TransferRestrictedRedeemableERC721ACommonFake(admin, steerer);
        voucher = RedeemableERC721ACommonFake(address(impl));
        redeemer = new BasicSingleRecordedRedeemer();
    }

    function _beforeSetup() internal virtual override {
        vm.prank(steerer);
        impl.setTransferRestriction(TransferRestriction.None);
    }

    function _afterSetup() internal virtual override {
        vm.prank(steerer);
        impl.setTransferRestriction(_testedRestriction());
    }

    function _modifyTestCase(TestCase memory tt) internal virtual override {
        if (tt.err.length == 0) {
            tt.err = abi.encodeWithSelector(
                ERC721ATransferRestrictedBase.DisallowedByTransferRestriction.selector, _testedRestriction()
            );
        }
    }

    function _testedRestriction() internal pure virtual returns (TransferRestriction);

    // We skip this test since we've already verify the correct revert for one redemption
    function testRedeemTwice(address alice, uint8 voucherId) public virtual override {}
}

contract TransferRestrictedRedeemableERC721ACommonFake_OnlyMint_RedeemTest is
    TransferRestrictedRedeemableERC721ACommonFake_RedeemTestWithRestriction
{
    function _testedRestriction() internal pure virtual override returns (TransferRestriction) {
        return TransferRestriction.OnlyMint;
    }
}

contract TransferRestrictedRedeemableERC721ACommonFake_Frozen_RedeemTest is
    TransferRestrictedRedeemableERC721ACommonFake_RedeemTestWithRestriction
{
    function _testedRestriction() internal pure virtual override returns (TransferRestriction) {
        return TransferRestriction.Frozen;
    }
}
