// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.0 <0.9.0;

import {IRedeemableToken} from "proof/redemption/interfaces/IRedeemableToken.sol";

import {ISellable} from "proof/sellers/interfaces/ISellable.sol";
import {Seller} from "proof/sellers/base/Seller.sol";
import {ImmutableSellableCallbacker, SellableCallbacker} from "proof/sellers/base/SellableCallbacker.sol";
import {ExactInternallyPriced, ExactFixedPrice} from "proof/sellers/base/InternallyPriced.sol";

import {PurchaseByProjectIDLib} from "proof/sellers/sellable/SellableERC721ACommonByProjectID.sol";

/**
 * @notice Seller that redeems a mint pass.
 */
abstract contract MintPassRedeemer is Seller, ImmutableSellableCallbacker {
    /**
     * @notice Emitted when the callback to the `IRedeemableToken` contract fails.
     */
    error RedeemableCallbackFailed(IRedeemableToken token, uint256 passId, bytes reason);

    /**
     * @notice The regular Grails Exhibition pass.
     */
    IRedeemableToken public immutable pass;

    constructor(ISellable sellable_, IRedeemableToken pass_) ImmutableSellableCallbacker(sellable_) {
        pass = pass_;
    }

    /**
     * @notice Redeems the given passes and redemptions pieces in the Grails Exhibition.
     */
    function _redeem(uint256[] memory passIds, bytes memory purchasePayload) internal {
        uint256 num = passIds.length;

        for (uint256 i = 0; i < num; ++i) {
            try pass.redeem(msg.sender, passIds[i]) {}
            catch (bytes memory reason) {
                revert RedeemableCallbackFailed(pass, passIds[i], reason);
            }
        }

        _purchase(msg.sender, uint64(num), /* total cost */ 0, purchasePayload);
    }
}

/**
 * @notice Seller that redeems a mint pass for a certain project on the sellable.
 */
abstract contract MintPassForProjectIDRedeemer is MintPassRedeemer {
    constructor(ISellable sellable_, IRedeemableToken pass_) MintPassRedeemer(sellable_, pass_) {}

    /**
     * @notice The struct encoding a redemption.
     * @param passId The ID of the pass to redeem.
     * @param projectId The ID of the project to redeem the pass for.
     */
    struct Redemption {
        uint256 passId;
        uint128 projectId;
    }

    /**
     * @notice Redeems the given passes for the specified projects on the sellable.
     */
    function _redeem(Redemption[] calldata redemptions) internal {
        uint256[] memory passIds = new uint256[](redemptions.length);
        uint128[] memory projectIds = new uint128[](redemptions.length);

        for (uint256 i = 0; i < redemptions.length; ++i) {
            passIds[i] = redemptions[i].passId;
            projectIds[i] = redemptions[i].projectId;
        }

        _redeem(passIds, PurchaseByProjectIDLib.encodePurchaseData(projectIds));
    }
}

/**
 * @notice Seller that redeems a mint pass for a certain project on the sellable.
 */
contract FreeMintPassForProjectIDRedeemer is MintPassForProjectIDRedeemer {
    constructor(ISellable sellable_, IRedeemableToken pass_) MintPassForProjectIDRedeemer(sellable_, pass_) {}

    function redeem(Redemption[] calldata redemptions) external {
        _redeem(redemptions);
    }
}

/**
 * @notice Seller that redeems a mint pass and an additional fee for a certain project on the sellable.
 */
contract FixedPricedMintPassForProjectIDRedeemer is ExactFixedPrice, MintPassForProjectIDRedeemer {
    constructor(ISellable sellable_, IRedeemableToken pass_, uint256 price)
        MintPassForProjectIDRedeemer(sellable_, pass_)
        ExactFixedPrice(price)
    {}

    /**
     * @dev Inheritance resoultion ensuring that the seller requires the correct `cost`.
     */
    function _checkAndModifyPurchase(address to, uint64 num, uint256 cost_, bytes memory data)
        internal
        view
        virtual
        override(Seller, ExactInternallyPriced)
        returns (address, uint64, uint256)
    {
        return ExactInternallyPriced._checkAndModifyPurchase(to, num, cost_, data);
    }

    /**
     * @notice Redeems the given passes and an additional fee for the specified projects on the sellable.
     * @dev Reverts if the value sent is not equal to the `cost` (i.e. `price * redemptions.length`).
     */
    function redeem(Redemption[] calldata redemptions) external payable {
        _redeem(redemptions);
    }
}
