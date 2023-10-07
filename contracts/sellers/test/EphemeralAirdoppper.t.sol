// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.

pragma solidity >=0.8.0 <0.9.0;

import {ERC721Fake} from "proof/constants/Testing.sol";
import {SellerTest} from "proof/sellers/../test/SellerTest.sol";

import {computeEphemeralAirdropperAddress, EphemeralAirdropper} from "../src/presets/EphemeralAirdropper.sol";

contract EphermeralAirdropTest is SellerTest {
    function testDeterministicAddress(bytes32 salt, address deployer, EphemeralAirdropper.Airdrop[] memory airdrops)
        public
    {
        vm.prank(deployer);
        EphemeralAirdropper airdropper = new EphemeralAirdropper{salt: salt}(sellable, airdrops);

        assertEq(computeEphemeralAirdropperAddress(salt, deployer, sellable, airdrops), address(airdropper));
    }

    function testSuccessManual() public {
        address alice = makeAddr("alice");
        address bob = makeAddr("bob");

        EphemeralAirdropper.Airdrop[] memory airdrops = new EphemeralAirdropper.Airdrop[](3);
        airdrops[0] = EphemeralAirdropper.Airdrop({to: alice, num: 2});
        airdrops[1] = EphemeralAirdropper.Airdrop({to: bob, num: 2});
        airdrops[2] = EphemeralAirdropper.Airdrop({to: alice, num: 1});

        bytes32 salt = bytes32(0);
        address airdropper = computeEphemeralAirdropperAddress(salt, address(this), sellable, airdrops);
        expectSellablePurchaseHandled({seller: airdropper, value: 0, to: alice, num: 2, data: ""});
        expectSellablePurchaseHandled({seller: airdropper, value: 0, to: bob, num: 2, data: ""});
        expectSellablePurchaseHandled({seller: airdropper, value: 0, to: alice, num: 1, data: ""});

        // `sellable` of `SellerTest` is not role-gated, so we don't need to set any permissions here
        new EphemeralAirdropper{salt: salt}(sellable, airdrops);

        assertEq(sellable.totalNumItems(), 5);
        assertEq(sellable.numItems(alice), 3);
        assertEq(sellable.numItems(bob), 2);
    }

    mapping(address => uint256) public wantBalances;

    function testFuzzed(EphemeralAirdropper.Airdrop[] memory airdrops) public {
        uint256 total;
        bytes32 salt = bytes32(0);
        address airdropper = computeEphemeralAirdropperAddress(salt, address(this), sellable, airdrops);
        for (uint256 i = 0; i < airdrops.length; i++) {
            total += airdrops[i].num;
            wantBalances[airdrops[i].to] += airdrops[i].num;
            expectSellablePurchaseHandled({
                seller: airdropper,
                value: 0,
                to: airdrops[i].to,
                num: airdrops[i].num,
                data: ""
            });
        }

        new EphemeralAirdropper{salt: salt}(sellable, airdrops);

        assertEq(sellable.totalNumItems(), total);
        for (uint256 i = 0; i < airdrops.length; i++) {
            assertEq(sellable.numItems(airdrops[i].to), wantBalances[airdrops[i].to]);
        }
    }
}
