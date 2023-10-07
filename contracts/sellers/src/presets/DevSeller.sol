// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity >=0.8.0 <0.9.0;

import {ISellable} from "../interfaces/ISellable.sol";

/**
 * @notice Seller for free of charge purchases. Only intended for testnet use.
 */
contract DevSeller {
    function purchase(ISellable sellable, address to, uint64 num, bytes memory data) public {
        sellable.handleSale(to, num, data);
    }

    function purchase(ISellable sellable, address to, uint64 num) public {
        purchase(sellable, to, num, bytes(""));
    }
}
