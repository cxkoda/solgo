// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.16;

contract Demo {
    function echo(string memory payload) external pure returns (string memory) {
        return string(abi.encodePacked("Solidity: ", payload));
    }
}
