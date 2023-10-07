// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {UUPSUpgradeable} from "openzeppelin-contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

/**
 * @notice An upgradeable contract implementation that can only be upgraded by its initializer.
 * @dev This is intended as base implementation to initialize UUPS proxies to make the proxy contract's initialization
 * code deterministic.
 */
contract OnlyUpgradeable is UUPSUpgradeable {
    error NotDeployer(address sender, address deployer);

    constructor() {
        _disableInitializers();
    }

    function initialize() public virtual initializer {
        __UUPSUpgradeable_init();

        // Using tx.origin since the intended deployer is an EOA and we might want to run this through a factory
        // contract.
        _changeAdmin(tx.origin);
    }

    function _authorizeUpgrade(address) internal virtual override {
        if (msg.sender != _getAdmin()) {
            revert NotDeployer(msg.sender, _getAdmin());
        }
    }
}
