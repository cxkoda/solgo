// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.0 <0.9.0;

import {Math} from "openzeppelin-contracts/utils/math/Math.sol";
import {IERC721} from "openzeppelin-contracts/token/ERC721/IERC721.sol";

import {ISellable} from "proof/sellers/interfaces/ISellable.sol";
import {Seller} from "proof/sellers/base/Seller.sol";
import {ImmutableSellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

interface IERC721WithTotalSupply is IERC721 {
    function totalSupply() external view returns (uint256);
}

/**
 * @notice Airdrops sellable tokens to the holders of another, non-burnable token contract.
 */
contract TokenHolderAirdropper is Seller, ImmutableSellableCallbacker {
    /**
     * @notice The token to airdrop to.
     */
    IERC721WithTotalSupply public immutable token;

    /**
     * @notice The number of sellable tokens that have been airdropped.
     */
    uint256 public numAirdropped;

    constructor(IERC721WithTotalSupply token_, ISellable sellable_) ImmutableSellableCallbacker(sellable_) {
        token = token_;
    }

    /**
     * @notice Airdrops a batch of sellable tokens to the holders of `token`.
     * @dev Iterates sequentially through the token IDs of `token`, picking up where the previous call left off, and
     * airdrops a sellable token to each holder.
     * @dev This function assumes that `token` is not burnable.
     * @param num The number of tokens to airdrop.
     */
    function airdrop(uint256 num) public virtual {
        uint256 tokenId = numAirdropped;
        // using the total supply us upper bound for tokenID assumes that `token` is not burnable.
        uint256 endTokenId = Math.min(tokenId + num, token.totalSupply());
        numAirdropped = endTokenId;

        while (tokenId < endTokenId) {
            _purchase(token.ownerOf(tokenId), 1, /* total cost */ 0, "");
            unchecked {
                ++tokenId;
            }
        }
    }
}
