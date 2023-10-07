// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "proof/constants/Testing.sol";
import {IERC721A} from "ERC721A/interfaces/IERC721A.sol";

import {IRedeemableToken} from "../src/interfaces/IRedeemableToken.sol";
import {BasicSingleRecordedRedeemer} from "../src/BasicSingleRecordedRedeemer.sol";
import {ERC721ACommon, RedeemableERC721ACommon} from "../src/voucher/RedeemableERC721ACommon.sol";

contract RedeemableERC721ACommonFake is RedeemableERC721ACommon {
    constructor(address admin, address steerer)
        ERC721ACommon(admin, steerer, "", "", payable(address(0x000FEE00)), 750)
    {}

    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }
}

contract RedeemableERC721ACommonTest is ProofTest {
    RedeemableERC721ACommonFake public voucher;
    BasicSingleRecordedRedeemer public redeemer;

    function setUp() public virtual {
        voucher = new RedeemableERC721ACommonFake(admin, steerer);
        redeemer = new BasicSingleRecordedRedeemer();
    }
}

contract RedeemTest is RedeemableERC721ACommonTest {
    enum Approval {
        None,
        Approve,
        ApproveForAll
    }

    struct TestCase {
        address owner;
        uint256 numTokensOwned;
        bool redeemerApproved;
        address sender;
        Approval senderApproval;
        uint256 voucherId;
        bytes err;
    }

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);

    function _modifyTestCase(TestCase memory tt) internal virtual {}

    function _beforeSetup() internal virtual {}

    function _afterSetup() internal virtual {}

    function _test(TestCase memory tt) internal {
        _modifyTestCase(tt);

        vm.assume(tt.owner != address(0));
        vm.assume(tt.sender != address(0));

        _beforeSetup();

        vm.startPrank(steerer);
        if (tt.redeemerApproved) {
            voucher.grantRole(voucher.REDEEMER_ROLE(), address(redeemer));
        }
        vm.stopPrank();

        voucher.mint(tt.owner, tt.numTokensOwned);
        uint256 supply = voucher.totalSupply();

        _afterSetup();

        if (tt.senderApproval == Approval.Approve) {
            vm.prank(tt.owner);
            voucher.approve(tt.sender, tt.voucherId);
        }
        if (tt.senderApproval == Approval.ApproveForAll) {
            vm.prank(tt.owner);
            voucher.setApprovalForAll(tt.sender, true);
        }

        bool fails = tt.err.length > 0;
        if (fails) {
            vm.expectRevert(tt.err);
        } else {
            vm.expectEmit(true, true, true, true, address(voucher));
            // to == 0 <=> burn
            emit Transfer(tt.owner, address(0), tt.voucherId);
        }

        vm.prank(tt.sender);
        redeemer.redeem(voucher, tt.voucherId);

        assertEq(voucher.totalSupply(), fails ? supply : supply - 1);
    }

    function testRedeem(address alice, uint8 voucherId) public {
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 256,
                redeemerApproved: true,
                sender: alice,
                senderApproval: Approval.None,
                voucherId: voucherId,
                err: hex""
            })
        );
    }

    function testRedeemTwice(address alice, uint8 voucherId) public virtual {
        TestCase memory tt = TestCase({
            owner: alice,
            numTokensOwned: 256,
            redeemerApproved: true,
            sender: alice,
            senderApproval: Approval.None,
            voucherId: voucherId,
            err: hex""
        });
        _test(tt);
        tt.err = abi.encodeWithSelector(IERC721A.OwnerQueryForNonexistentToken.selector);
        _test(tt);
    }

    function testRedeemNonexistent(address alice, uint8 voucherId) public {
        vm.assume(voucherId > 0);
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 1,
                redeemerApproved: true,
                sender: alice,
                senderApproval: Approval.None,
                // Since we only mint 1 token and have voucherId > 0, this will
                // always fail.
                voucherId: voucherId,
                err: abi.encodeWithSelector(IERC721A.OwnerQueryForNonexistentToken.selector)
            })
        );
    }

    function testRedeemerNotApproved(address alice, uint8 voucherId) public {
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 256,
                redeemerApproved: false,
                sender: alice,
                senderApproval: Approval.None,
                voucherId: voucherId,
                err: missingRoleError(address(redeemer), voucher.REDEEMER_ROLE())
            })
        );
    }

    function testApprovedSender(address alice, address bob, uint8 voucherId) public {
        vm.assume(alice != bob);
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 256,
                redeemerApproved: true,
                sender: bob,
                senderApproval: Approval.Approve,
                voucherId: voucherId,
                err: hex""
            })
        );
    }

    function testApprovedForAllSender(address alice, address bob, uint8 voucherId) public {
        vm.assume(alice != bob);
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 256,
                redeemerApproved: true,
                sender: bob,
                senderApproval: Approval.ApproveForAll,
                voucherId: voucherId,
                err: hex""
            })
        );
    }

    function testSenderNotApproved(address alice, address bob, uint8 voucherId) public {
        vm.assume(alice != bob);
        _test(
            TestCase({
                owner: alice,
                numTokensOwned: 256,
                redeemerApproved: true,
                sender: bob,
                senderApproval: Approval.None,
                voucherId: voucherId,
                err: abi.encodeWithSelector(
                    IRedeemableToken.RedeemerCallerNotAllowedToSpendVoucher.selector, bob, voucherId
                    )
            })
        );
    }
}
