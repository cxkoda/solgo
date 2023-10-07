// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {IEntropyOracle} from "./EntropyOracle.sol";

contract EntropyOracleStub is IEntropyOracle {
    mapping(uint256 => bytes32) private _blockEntropy;

    function requestEntropy() public {
        requestEntropy(block.number);
    }

    function requestEntropy(uint256 blockNumber) public {
        if (_blockEntropy[blockNumber] == 0) {
            emit EntropyRequested(blockNumber);
            _blockEntropy[blockNumber] = bytes32(uint256(1));
        }
    }

    function blockEntropy(uint256 blockNumber) public view virtual returns (bytes32) {
        bytes32 entropy = _blockEntropy[blockNumber];
        if (uint256(entropy) > 1) {
            return entropy;
        }
        return 0;
    }

    function provideEntropy(uint256 blockNumber, bytes32 entropy) public {
        _blockEntropy[blockNumber] = entropy;
    }
}
