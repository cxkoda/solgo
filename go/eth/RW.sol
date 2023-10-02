// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @dev For testing of clients that use different read and write connections.
contract RW {
    string private _payload;

    function read() external view returns (string memory) {
        return _payload;
    }

    function write(string memory x) external {
        _payload = x;
    }
}
