// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {Initializable} from "openzeppelin-contracts-upgradeable/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "openzeppelin-contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

/**
 * @notice A dummy contract to test UUPS upgradeability.
 * @dev This contract MUST NOT be deployed to production since there is no upgrade authorisation in place.
 * It is intended to be upgraded to and rolled back to the previous implementation immediately by calling
 * `upgradeToAndCall` with a call to `upgradeTo(previousImpl)` as setup calldata.
 */
contract UUPSDummy is Initializable, UUPSUpgradeable {
    constructor() {
        _disableInitializers();
    }

    function _authorizeUpgrade(address newImplementation) internal virtual override {}
}
