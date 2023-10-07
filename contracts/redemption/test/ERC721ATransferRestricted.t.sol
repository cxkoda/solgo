// SPDX-License-Identifier: MIT
// Copyright 2022 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "proof/constants/Testing.sol";

import {IERC721A} from "ERC721A/IERC721A.sol";
import {ERC721ACommon} from "ethier/erc721/ERC721ACommon.sol";
import {
    ERC721ATransferRestricted,
    ERC721ATransferRestrictedBase,
    TransferRestriction
} from "../src/restricted/ERC721ATransferRestricted.sol";

contract TestableERC721ATransferRestricted is ERC721ATransferRestricted {
    constructor(address admin, address steerer) ERC721ACommon(admin, steerer, "", "", payable(address(0xfee)), 0) {}

    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }

    function burn(uint256 tokenId) public {
        _burn(tokenId);
    }
}

contract ERC721ATransferRestrictedGeneralTest is ProofTest {
    TestableERC721ATransferRestricted public token;

    function setUp() public virtual {
        token = new TestableERC721ATransferRestricted(admin, steerer);
    }

    function testVandalCannotCallOwnerFunctions(address vandal, uint8 restriction) public {
        TransferRestriction r = toTransferRestriction(restriction);

        expectRevertNotSteererThenPrank(vandal);
        token.setTransferRestriction(r);

        expectRevertNotSteererThenPrank(vandal);
        token.lockTransferRestriction(r);
    }

    function toTransferRestriction(uint256 x) public view returns (TransferRestriction) {
        return TransferRestriction(
            bound(x, uint256(type(TransferRestriction).min), uint256(type(TransferRestriction).max))
        );
    }

    function testRestrictionGetter() public {
        for (uint8 i = uint8(type(TransferRestriction).min); i < uint8(type(TransferRestriction).max); ++i) {
            vm.prank(steerer);
            token.setTransferRestriction(TransferRestriction(i));

            assertEq(uint8(token.transferRestriction()), i);
        }
    }

    function testDefaultRestriction() public {
        assertEq(uint8(token.transferRestriction()), uint8(TransferRestriction.None));
    }

    function testLock(uint8 setRestriction, uint8 lockRestriction) public {
        TransferRestriction setTo = toTransferRestriction(setRestriction);
        TransferRestriction lockAs = toTransferRestriction(lockRestriction);

        vm.prank(steerer);
        token.setTransferRestriction(setTo);

        if (setTo != lockAs) {
            vm.expectRevert(
                abi.encodeWithSelector(ERC721ATransferRestricted.TransferRestrictionCheckFailed.selector, setTo)
            );
        }
        vm.prank(steerer);
        token.lockTransferRestriction(lockAs);

        if (setTo == lockAs) {
            vm.expectRevert(abi.encodeWithSelector(ERC721ATransferRestricted.TransferRestrictionLocked.selector));
        }
        vm.prank(steerer);
        token.setTransferRestriction(setTo);
    }
}

contract TransferBehaviourTest is ProofTest {
    TestableERC721ATransferRestricted public token;

    bytes internal _lockedErr;
    bytes internal _notApprovedErr = abi.encodeWithSelector(IERC721A.TransferCallerNotOwnerNorApproved.selector);

    struct TestCase {
        TransferRestriction restriction;
        bool wantTransfersLocked;
        bool wantMintLocked;
        bool wantBurnLocked;
    }

    TestCase public tt;

    constructor(TestCase memory testCase_) {
        tt = testCase_;
        _lockedErr = abi.encodeWithSelector(
            ERC721ATransferRestrictedBase.DisallowedByTransferRestriction.selector, tt.restriction
        );
    }

    function setUp() public virtual {
        token = new TestableERC721ATransferRestricted(admin, steerer);
    }

    function _checkAndSetup(address alice, address bob, uint8 tokenId) internal {
        vm.assume(alice != bob);
        vm.assume(alice != address(0));
        vm.assume(bob != address(0));

        token.mint(alice, uint256(tokenId) + 1);

        vm.prank(steerer);
        token.setTransferRestriction(tt.restriction);
    }

    function testOwnerTransfer(address alice, address bob, uint8 tokenId) public {
        _checkAndSetup(alice, bob, tokenId);
        bool locked = tt.wantTransfersLocked;

        if (locked) {
            vm.expectRevert(_lockedErr);
        }
        vm.prank(alice);
        token.transferFrom(alice, bob, tokenId);

        assertEq(token.balanceOf(alice), token.totalSupply() - (locked ? 0 : 1));
        assertEq(token.balanceOf(bob), locked ? 0 : 1);
    }

    function testApprovedTransfer(address alice, address bob, uint8 tokenId) public {
        _checkAndSetup(alice, bob, tokenId);
        bool locked = tt.wantTransfersLocked;

        vm.prank(alice);
        token.approve(bob, tokenId);

        if (locked) {
            vm.expectRevert(_lockedErr);
        }
        vm.prank(bob);
        token.transferFrom(alice, bob, tokenId);

        assertEq(token.balanceOf(alice), token.totalSupply() - (locked ? 0 : 1));
        assertEq(token.balanceOf(bob), locked ? 0 : 1);
    }

    function testApprovedForAllTransfer(address alice, address bob, uint8 tokenId) public {
        _checkAndSetup(alice, bob, tokenId);
        bool locked = tt.wantTransfersLocked;

        vm.prank(alice);
        token.setApprovalForAll(bob, true);

        if (locked) {
            vm.expectRevert(_lockedErr);
        }
        vm.prank(bob);
        token.transferFrom(alice, bob, tokenId);

        assertEq(token.balanceOf(alice), token.totalSupply() - (locked ? 0 : 1));
        assertEq(token.balanceOf(bob), locked ? 0 : 1);
    }

    function testMint(address alice, uint8 num) public {
        vm.assume(alice != address(0));
        vm.assume(num > 0);

        vm.prank(steerer);
        token.setTransferRestriction(tt.restriction);

        uint256 totalSupply = token.totalSupply();

        bool locked = tt.wantMintLocked;
        if (locked) {
            vm.expectRevert(_lockedErr);
        }
        token.mint(alice, num);

        assertEq(token.balanceOf(alice), totalSupply + (locked ? 0 : num));
    }

    function testBurn(address alice, address bob, uint8 tokenId) public {
        _checkAndSetup(alice, bob, tokenId);
        uint256 totalSupply = token.totalSupply();
        bool locked = tt.wantBurnLocked;

        if (locked) {
            vm.expectRevert(_lockedErr);
        }
        token.burn(tokenId);

        assertEq(token.balanceOf(alice), totalSupply - (locked ? 0 : 1));
    }
}

contract NoneRestrictionTest is TransferBehaviourTest {
    constructor()
        TransferBehaviourTest(
            TransferBehaviourTest.TestCase({
                restriction: TransferRestriction.None,
                wantTransfersLocked: false,
                wantMintLocked: false,
                wantBurnLocked: false
            })
        )
    {}
}

contract OnlyMintRestrictionTest is TransferBehaviourTest {
    constructor()
        TransferBehaviourTest(
            TransferBehaviourTest.TestCase({
                restriction: TransferRestriction.OnlyMint,
                wantTransfersLocked: true,
                wantMintLocked: false,
                wantBurnLocked: true
            })
        )
    {}
}

contract OnlyBurnRestrictionTest is TransferBehaviourTest {
    constructor()
        TransferBehaviourTest(
            TransferBehaviourTest.TestCase({
                restriction: TransferRestriction.OnlyBurn,
                wantTransfersLocked: true,
                wantMintLocked: true,
                wantBurnLocked: false
            })
        )
    {}
}

contract FrozenRestrictionTest is TransferBehaviourTest {
    constructor()
        TransferBehaviourTest(
            TransferBehaviourTest.TestCase({
                restriction: TransferRestriction.Frozen,
                wantTransfersLocked: true,
                wantMintLocked: true,
                wantBurnLocked: true
            })
        )
    {}
}
