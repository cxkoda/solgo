// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @notice A contract for testing Firehose integrations.
contract Emitter {
    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);

    event WithData(uint8 indexed topic, bytes data);

    function transfer(address from, address to, uint256 tokenId) external {
        emit Transfer(from, to, tokenId);
    }

    function withData(uint8 topic, bytes calldata data) external {
        emit WithData(topic, data);
    }
}
