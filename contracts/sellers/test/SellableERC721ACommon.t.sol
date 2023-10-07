// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {ProofTest} from "proof/constants/Testing.sol";
import {ISellable} from "proof/sellers/interfaces/ISellable.sol";
import {ERC721ACommon, SellableERC721ACommon} from "proof/sellers/sellable/SellableERC721ACommon.sol";
import {IERC721Events} from "proof/constants/events/IERC721Events.sol";
import {IERC721} from "openzeppelin-contracts/interfaces/IERC721.sol";

contract TestableSellableERC721ACommon is SellableERC721ACommon {
    constructor(address admin, address steerer)
        ERC721ACommon(admin, steerer, "Sellable", "SLBL", payable(address(0xFEE)), 0)
    {}
}

contract SellableERC721ACommonTest is ProofTest {
    TestableSellableERC721ACommon public sellable;

    address public immutable seller = makeAddr("seller");

    function newSellableERC721() public virtual returns (address) {
        return address(new TestableSellableERC721ACommon(admin, steerer));
    }

    function setUp() public virtual {
        sellable = TestableSellableERC721ACommon(newSellableERC721());

        vm.startPrank(steerer);
        sellable.grantRole(sellable.AUTHORISED_SELLER_ROLE(), seller);
        vm.stopPrank();
    }
}

contract VandalTest is SellableERC721ACommonTest {
    function testCannotHandleSale(address vandal, address to, uint64 num, bytes memory data) public {
        vm.assume(vandal != seller);
        vm.expectRevert(missingRoleError(vandal, sellable.AUTHORISED_SELLER_ROLE()));
        vm.prank(vandal, seller);
        sellable.handleSale(to, num, data);
    }

    function testCannotLock(address vandal) public virtual {
        vm.assume(vandal != steerer);
        vm.expectRevert(missingRoleError(vandal, sellable.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        sellable.lockSellers();
    }
}

contract LockSellersTest is SellableERC721ACommonTest {
    // Copying this here because we deliberately did not expose the role in the contract.
    bytes32 public immutable noop = keccak256("NOOP_ROLE");
    bytes32 public sellerRole;

    function setUp() public virtual override {
        super.setUp();
        sellerRole = sellable.AUTHORISED_SELLER_ROLE();
    }

    function testCannotGrantNoopRole(address grantee) public {
        vm.prank(admin);
        vm.expectRevert(missingRoleError(admin, noop));
        sellable.grantRole(noop, grantee);

        vm.prank(steerer);
        vm.expectRevert(missingRoleError(steerer, noop));
        sellable.grantRole(noop, grantee);
    }

    function testCannotGrantNoopRole(address granter, address grantee) public {
        vm.prank(granter);
        vm.expectRevert(missingRoleError(granter, noop));
        sellable.grantRole(noop, grantee);
    }

    function testCannotAddSellersAfterLocking(address newSeller) public {
        vm.assume(newSeller != seller);
        vm.startPrank(steerer);
        sellable.grantRole(sellerRole, newSeller);
        assertTrue(sellable.hasRole(sellerRole, newSeller));

        sellable.revokeRole(sellerRole, newSeller);
        assertFalse(sellable.hasRole(sellerRole, newSeller));

        sellable.lockSellers();

        vm.expectRevert(missingRoleError(steerer, noop));
        sellable.grantRole(sellerRole, newSeller);

        assertFalse(sellable.hasRole(sellerRole, newSeller));
    }

    function testCannotRemoveSellersAfterLocking() public {
        vm.startPrank(steerer);
        sellable.revokeRole(sellerRole, seller);
        assertFalse(sellable.hasRole(sellerRole, seller));

        sellable.grantRole(sellerRole, seller);
        assertTrue(sellable.hasRole(sellerRole, seller));

        sellable.lockSellers();

        vm.expectRevert(missingRoleError(steerer, noop));
        sellable.revokeRole(sellerRole, seller);

        assertTrue(sellable.hasRole(sellerRole, seller));
    }

    function testLockingTwiceHasNoEffect(address newSeller) public {
        vm.assume(newSeller != seller);
        vm.startPrank(steerer);
        sellable.lockSellers();
        sellable.lockSellers();

        vm.expectRevert(missingRoleError(steerer, noop));
        sellable.grantRole(sellerRole, newSeller);

        vm.expectRevert(missingRoleError(steerer, noop));
        sellable.revokeRole(sellerRole, seller);

        assertFalse(sellable.hasRole(sellerRole, newSeller));
        assertTrue(sellable.hasRole(sellerRole, seller));
    }
}

contract HandleSaleTest is SellableERC721ACommonTest, IERC721Events {
    struct TestCase {
        address caller;
        address to;
        uint64 num;
        uint256 value;
    }

    function _test(TestCase memory tt, bytes memory err) internal {
        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            uint256 totalSupply = sellable.totalSupply();
            for (uint256 i; i < tt.num; ++i) {
                vm.expectEmit(true, true, true, true, address(sellable));
                emit Transfer(address(0), tt.to, totalSupply + i);
            }
        }

        vm.deal(tt.caller, tt.value);
        vm.prank(tt.caller);
        sellable.handleSale{value: tt.value}(tt.to, tt.num, "");
    }

    struct Fuzz {
        address to;
        uint8 num;
        uint256 value;
    }

    function _happyCase(Fuzz memory fuzz) internal view returns (TestCase memory) {
        vm.assume(fuzz.num > 0);
        _assumeNotContract(fuzz.to);
        return TestCase({caller: seller, to: fuzz.to, num: fuzz.num, value: fuzz.value});
    }

    function testHappy(Fuzz memory fuzz) public {
        _test(_happyCase(fuzz), "");
    }

    function testHappyAfterLocking(Fuzz memory fuzz) public virtual {
        vm.prank(steerer);
        sellable.lockSellers();
        testHappy(fuzz);
    }

    function testUnapprovedAfterRevoke(Fuzz memory fuzz) public {
        vm.startPrank(steerer);
        sellable.revokeRole(sellable.AUTHORISED_SELLER_ROLE(), seller);
        vm.stopPrank();

        assertFalse(sellable.hasRole(sellable.AUTHORISED_SELLER_ROLE(), seller));
        _test(_happyCase(fuzz), missingRoleError(seller, sellable.AUTHORISED_SELLER_ROLE()));
    }

    function testApproveNewSeller(Fuzz memory fuzz, address newSeller) public {
        vm.assume(newSeller != address(0));
        vm.assume(newSeller != seller);

        TestCase memory tt = _happyCase(fuzz);
        tt.caller = newSeller;

        assertFalse(sellable.hasRole(sellable.AUTHORISED_SELLER_ROLE(), newSeller));
        _test(tt, missingRoleError(newSeller, sellable.AUTHORISED_SELLER_ROLE()));

        vm.startPrank(steerer);
        sellable.grantRole(sellable.AUTHORISED_SELLER_ROLE(), newSeller);
        vm.stopPrank();

        assertEq(sellable.hasRole(sellable.AUTHORISED_SELLER_ROLE(), newSeller), true);
        _test(tt, "");
    }
}
