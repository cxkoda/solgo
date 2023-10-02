// SPDX-License-Identifier: MIT
pragma solidity >=0.8.0 <0.9.0;

import {ERC721A} from "ERC721A/ERC721A.sol";

contract TestableERC721 is ERC721A("", "") {
    function mintN(uint256 n) external {
        _mint(msg.sender, n);
    }
}
