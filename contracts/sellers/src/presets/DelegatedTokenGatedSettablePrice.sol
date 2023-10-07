// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.16;

import {ISellable, CallbackerWithAccessControl} from "./CallbackerWithAccessControl.sol";
import {ExactSettableFixedPrice} from "./ExactSettableFixedPrice.sol";
import {InternallyPriced, ExactInternallyPriced} from "../base/InternallyPriced.sol";

import {IERC721, IDelegationRegistry, DelegatedTokenGated} from "../mechanics/TokenGated.sol";

/**
 * @notice Public seller with a fixed price.
 */
contract DelegatedTokenGatedSettablePrice is DelegatedTokenGated, ExactSettableFixedPrice {
    constructor(
        address admin,
        address steerer,
        ISellable sellable_,
        uint256 price,
        IERC721 gatingToken,
        IDelegationRegistry registry
    )
        CallbackerWithAccessControl(admin, steerer, sellable_)
        DelegatedTokenGated(gatingToken, registry)
        ExactSettableFixedPrice(price)
    {}

    function _checkAndModifyPurchase(address to, uint64 num, uint256 cost_, bytes memory data)
        internal
        view
        virtual
        override(InternallyPriced, ExactInternallyPriced)
        returns (address, uint64, uint256)
    {
        return ExactInternallyPriced._checkAndModifyPurchase(to, num, cost_, data);
    }
}
