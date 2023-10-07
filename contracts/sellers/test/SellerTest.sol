// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {ProofTest} from "proof/constants/Testing.sol";
import {ISellable} from "proof/sellers/interfaces/ISellable.sol";
import {PurchaseExecuter} from "proof/sellers/interfaces/PurchaseExecuter.sol";

interface SellableMockEvents {
    event SellablePurchaseHandled(
        address msgSender, uint256 indexed msgValue, address indexed to, uint64 indexed num, bytes data
    );
}

contract SellableMock is ISellable, SellableMockEvents {
    uint256 public totalNumItems;
    mapping(address => uint256) public numItems;

    function handleSale(address to, uint64 num, bytes calldata data) public payable override {
        emit SellablePurchaseHandled(msg.sender, msg.value, to, num, data);
        numItems[to] += num;
        totalNumItems += num;
    }
}

contract SellerTest is ProofTest, SellableMockEvents {
    uint256 public constant DEFAULT_PRICE = 1337 ether;

    SellableMock public sellable;

    constructor() {
        sellable = new SellableMock();
    }

    modifier assertTotalNumItemsIncreased(uint256 delta, bytes memory err) {
        uint256 numPurchasedBefore = sellable.totalNumItems();
        _;
        assertEq(
            sellable.totalNumItems(),
            numPurchasedBefore + zeroIfErrElse(err, delta),
            "sellable.numItems(to) not increased correctly"
        );
    }

    modifier assertNumItemsIncreased(address to, uint256 delta, bytes memory err) {
        uint256 numPurchasedBefore = sellable.numItems(to);
        _;
        assertEq(
            sellable.numItems(to),
            numPurchasedBefore + zeroIfErrElse(err, delta),
            "sellable.numItems(to) not increased correctly"
        );
    }

    struct ExpectedDelta {
        address to;
        uint256 delta;
    }

    mapping(address => uint256)[] private _addressDeltas;

    modifier assertMultipleNumItemsIncreased(ExpectedDelta[] memory changes, bytes memory err) {
        mapping(address => uint256) storage addressDelta = _addressDeltas.push();

        uint256[] memory numPurchasedBefore = new uint[](changes.length);
        for (uint256 i; i < changes.length; i++) {
            numPurchasedBefore[i] = sellable.numItems(changes[i].to);
            addressDelta[changes[i].to] += changes[i].delta;
        }
        _;
        bool fails = err.length > 0;
        for (uint256 i; i < changes.length; i++) {
            assertEq(
                sellable.numItems(changes[i].to),
                numPurchasedBefore[i] + (fails ? 0 : addressDelta[changes[i].to]),
                string.concat("sellable.numItems([to=", vm.toString(changes[i].to), "]) not increased correctly")
            );
        }
    }

    function expectSellablePurchaseHandled(uint256 value, address to, uint64 num) public {
        vm.expectEmit(true, true, true, false, address(sellable));
        emit SellablePurchaseHandled(address(0), value, to, num, "");
    }

    function expectSellablePurchaseHandled(address seller, uint256 value, address to, uint64 num, bytes memory data)
        public
    {
        vm.expectEmit(true, true, true, true, address(sellable));
        emit SellablePurchaseHandled(seller, value, to, num, data);
    }
}
