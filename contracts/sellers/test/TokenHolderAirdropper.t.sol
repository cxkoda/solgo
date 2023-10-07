// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.

pragma solidity >=0.8.0 <0.9.0;

import {ERC721Fake} from "proof/constants/Testing.sol";
import {SellerTest} from "proof/sellers/../test/SellerTest.sol";

import {IERC721WithTotalSupply, TokenHolderAirdropper} from "../src/presets/TokenHolderAirdropper.sol";

contract TokenHolderAirdropperTest is SellerTest {
    ERC721Fake public token;
    TokenHolderAirdropper public airdropper;

    function setUp() public {
        token = new ERC721Fake();
        airdropper = new TokenHolderAirdropper(IERC721WithTotalSupply(address(token)), sellable);
    }
}

contract AirdropTest is TokenHolderAirdropperTest {
    function _airdrop(uint256 num, uint256 wantIncreased) internal assertTotalNumItemsIncreased(wantIncreased, "") {
        uint256 startTokenId = airdropper.numAirdropped();
        for (uint256 i = 0; i < wantIncreased; i++) {
            expectSellablePurchaseHandled({
                seller: address(airdropper),
                value: 0,
                to: token.ownerOf(startTokenId + i),
                num: 1,
                data: ""
            });
        }

        airdropper.airdrop(num);
        assertEq(airdropper.numAirdropped(), startTokenId + wantIncreased);
    }

    function testSuccessManual() public {
        address alice = makeAddr("alice");
        address bob = makeAddr("bob");

        uint256 tokenId;
        token.mint(alice, tokenId++);
        token.mint(alice, tokenId++);
        token.mint(alice, tokenId++);
        token.mint(bob, tokenId++);
        token.mint(bob, tokenId++);
        token.mint(alice, tokenId++);

        _airdrop(2, 2);

        assertEq(sellable.numItems(alice), 2);
        assertEq(sellable.numItems(bob), 0);

        _airdrop(2, 2);
        assertEq(sellable.numItems(alice), 3);
        assertEq(sellable.numItems(bob), 1);

        _airdrop(3, 2);
        assertEq(sellable.numItems(alice), 4);
        assertEq(sellable.numItems(bob), 2);

        _airdrop(100, 0);
        assertEq(sellable.numItems(alice), 4);
        assertEq(sellable.numItems(bob), 2);
    }

    function testFuzzed(address[] calldata owners, uint256 totalSupply) public {
        vm.assume(owners.length > 0);
        vm.assume(owners.length < 20);
        for (uint256 i; i < owners.length; i++) {
            _assumeNotContract(owners[i]);
        }

        // speeding up tests
        totalSupply = bound(totalSupply, 1, 3000);

        for (uint256 i = 0; i < totalSupply; i++) {
            token.mint(owners[i % owners.length], i);
        }

        uint256 numBatches = 10;
        uint256 batchSize = totalSupply / numBatches;
        for (uint256 i = 0; i < numBatches; i++) {
            _airdrop(batchSize, batchSize);
        }

        uint256 remaining = totalSupply - batchSize * numBatches;
        _airdrop(remaining, remaining);
        _airdrop(100, 0);

        assertEq(airdropper.numAirdropped(), totalSupply);
        assertEq(sellable.totalNumItems(), totalSupply);

        for (uint256 i = 0; i < owners.length; i++) {
            assertEq(sellable.numItems(owners[i]), token.balanceOf(owners[i]));
        }
    }
}
