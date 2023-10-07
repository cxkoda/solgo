// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.0 <0.9.0;

import {Math} from "openzeppelin-contracts/utils/math/Math.sol";
import {Create2} from "openzeppelin-contracts/utils/Create2.sol";
import {IERC721} from "openzeppelin-contracts/token/ERC721/IERC721.sol";

import {ISellable} from "proof/sellers/interfaces/ISellable.sol";
import {Seller} from "proof/sellers/base/Seller.sol";
import {ImmutableSellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";

/**
 * @notice EphemeralAirdropper airdrops sellable tokens to a list of recipients.
 * @dev This contract has to be used with care as it deviates from the standard Seller framework for efficiency,
 * It is indended to be used with `computeEphemeralAirdropperAddress` to authorize the airdropper as seller before
 * deploying it.
 */
contract EphemeralAirdropper {
    struct Airdrop {
        address to;
        uint64 num;
    }

    constructor(ISellable sellable, Airdrop[] memory airdrops) {
        for (uint256 i; i < airdrops.length; ++i) {
            sellable.handleSale(airdrops[i].to, airdrops[i].num, "");
        }

        // Deliberately not selfdestructing to avoid the CREATE2 address from being reused which would allow replay
        // exploits if this is deployed via a factory. We could use additional logic here to prevent this (e.g. by
        // binding deployments to a certain tx.origin) but it is not worth the additional complexity.
    }
}

/**
 * @notice Computes the CREATE2 address of an EphemeralAirdropper contract.
 */
function computeEphemeralAirdropperAddress(
    bytes32 salt,
    address deployer,
    ISellable sellable,
    EphemeralAirdropper.Airdrop[] memory airdrops
) pure returns (address) {
    return Create2.computeAddress(
        salt,
        keccak256(abi.encodePacked(type(EphemeralAirdropper).creationCode, abi.encode(sellable, airdrops))),
        deployer
    );
}
