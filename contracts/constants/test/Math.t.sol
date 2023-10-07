// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {ProofTest} from "../src/Testing.sol";
import {Math} from "../src/Math.sol";

contract IntSqrtTest is ProofTest {
    function testSqrtOfSquares(uint128 x) public {
        uint256 root = Math.intSqrt(uint256(x) * uint256(x));
        assertEq(root, x);
    }

    function testSqrt(uint256 x) public {
        uint256 y = Math.intSqrt(x);
        assertLe(y * y, x);

        if (y < type(uint128).max) {
            assertGt((y + 1) * (y + 1), x);
        }
    }
}
