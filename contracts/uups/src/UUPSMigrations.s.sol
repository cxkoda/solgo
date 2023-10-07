// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.0;

import {console2} from "forge-std/console2.sol";
import {Script} from "forge-std/Script.sol";
import {VmSafe} from "forge-std/Vm.sol";

import {Address} from "openzeppelin-contracts/utils/Address.sol";
import {Strings} from "openzeppelin-contracts/utils/Strings.sol";
import {ERC1967Proxy} from "openzeppelin-contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {UUPSUpgradeable} from "openzeppelin-contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

import {UUPSDummy} from "./UUPSDummy.sol";
import {OnlyUpgradeable} from "./OnlyUpgradeable.sol";

/**
 * @notice Helper library to work with forge's default CREATE2 factory contract.
 * @dev We're handling this manually instead of letting forge do it's magic because there were some issues with
 * mismatching deployers while performing a dry-run vs broadcast.
 */
library Create2FactoryWorker {
    using Address for address;
    using Create2FactoryWorker for Deployment;

    /**
     * @notice The CREATE2 factory contract. See also https://github.com/Arachnid/deterministic-deployment-proxy
     */
    address internal constant CREATE2_FACTORY = 0x4e59b44847b379578588920cA78FbF26c0B4956C;

    /**
     * @notice Deploys the given init code via the factory contract using CREATE2.
     */
    function create2(bytes32 salt, bytes memory initCode_) internal returns (address) {
        bytes memory ret = CREATE2_FACTORY.functionCall(abi.encodePacked(salt, initCode_));
        address addr_;
        assembly {
            addr_ := shr(96, mload(add(ret, 0x20)))
        }
        return addr_;
    }

    /**
     * @notice Computes the deterministic CREATE2 contract address for a given init code hash.
     */
    function computeCreate2Address(bytes32 salt, bytes32 initCodeHash_) internal pure returns (address) {
        return
            address(uint160(uint256(keccak256(abi.encodePacked(bytes1(0xff), CREATE2_FACTORY, salt, initCodeHash_)))));
    }

    /**
     * @notice Performs an idempotent CREATE2 deployment, i.e. does nothing if the contract was already deployed.
     */
    function idempotentCreate2(bytes32 salt, bytes memory initCode_) internal returns (address) {
        address wantAddr = computeCreate2Address(salt, keccak256(initCode_));
        if (wantAddr.code.length > 0) {
            return wantAddr;
        }

        address addr_ = create2(salt, initCode_);
        require(addr_ == wantAddr, "create2 failed: address mismatch");
        require(addr_.code.length > 0, "create2 failed: no code");

        return addr_;
    }

    /**
     * @notice Encodes a deployment intended to be performed via the CREATE2 factory.
     * @params salt The salt to be used for the CREATE2 deployment.
     * @params creationCode The creation code of the contract to be deployed.
     * @params constructorArgs The constructor arguments of the contract to be deployed.
     * @params expectedInitCodeHash The expected init code hash of the contract to be deployed. This is intended as a
     * safety check to ensure that the deployment was specified correctly. To ignore this, call
     * `deployment.autoInitCodeHash()`.
     */
    struct Deployment {
        string label;
        bytes32 salt;
        bytes creationCode;
        bytes constructorArgs;
        bytes32 expectedInitCodeHash;
    }

    /**
     * @notice Computes the initialisation code for a given deployment.
     */
    function initCode(Deployment memory depl) internal pure returns (bytes memory) {
        return abi.encodePacked(depl.creationCode, depl.constructorArgs);
    }

    /**
     * @notice Computes the initialisation code hash for a given deployment.
     */
    function initCodeHash(Deployment memory depl) internal pure returns (bytes32) {
        return keccak256(depl.initCode());
    }

    /**
     * @notice Computes the deterministic CREATE2 address for a given deployment.
     * @dev The deployment does not need to be executed.
     */
    function addr(Deployment memory depl) internal pure returns (address) {
        depl.requireValidInitCodeHash();
        return computeCreate2Address(depl.salt, depl.initCodeHash());
    }

    /**
     * @notice Performs an idempotent CREATE2 deployment of the given deployment.
     */
    function idempotentCreate2(Deployment memory depl) internal returns (address) {
        depl.requireValidInitCodeHash();
        return idempotentCreate2(depl.salt, depl.initCode());
    }

    /**
     * @notice Checks if the given deployment has already been executed.
     */
    function isDeployed(Deployment memory depl) internal view returns (bool) {
        return depl.addr().code.length > 0;
    }

    /**
     * @notice Checks if the deployments init code hash matches the expected one. Reverts otherwise.
     */
    function requireValidInitCodeHash(Deployment memory depl) internal pure {
        require(
            depl.initCodeHash() == depl.expectedInitCodeHash,
            string.concat(
                "Create2FactoryWorker: unexpected init code hash for ",
                depl.label,
                ": actual ",
                Strings.toHexString(uint256(depl.initCodeHash()), 32),
                ", expected ",
                Strings.toHexString(uint256(depl.expectedInitCodeHash), 32)
            )
        );
    }

    /**
     * @notice Fills the expected init code hash of the deployment with the one computed from the construction details.
     * @dev This is a convenience wrapper to compute and set the deployments expected init code hash automatically. This
     * should not be
     * used by default but rather the expected hash should be set manually to guarantee that the contract bytecode did
     * not change so that all deployments remain idempotent.
     */
    function autoInitCodeHash(Deployment memory depl) internal pure returns (Deployment memory) {
        depl.expectedInitCodeHash = depl.initCodeHash();
        return depl;
    }
}

/**
 * @notice Helper library to work with UUPS deployments.
 */
library UpgradesLib {
    using Create2FactoryWorker for Create2FactoryWorker.Deployment;
    using UpgradesLib for Implementation;
    using UpgradesLib for Proxy;

    /**
     * @notice Encodes an UUPS implementation contract.
     * @param deployment The deployment details of the implementation contract.
     * @param setupCalldata The calldata to be run while upgrading a proxy to this implementation.
     */
    struct Implementation {
        Create2FactoryWorker.Deployment deployment;
        bytes setupCalldata;
    }

    /**
     * @notice Computes the CREATE2 address of the implementation contract.
     */
    function addr(Implementation memory impl) internal pure returns (address) {
        return impl.deployment.addr();
    }

    /**
     * @notice Performs an idempotent CREATE2 deployment of the implementation contract.
     */
    function idempotentCreate2(Implementation memory impl) internal returns (address) {
        return impl.deployment.idempotentCreate2();
    }

    /**
     * @notice Encodes a UUPS proxy contract.
     * @param deployment The deployment details of the proxy contract.
     */
    struct Proxy {
        Create2FactoryWorker.Deployment deployment;
    }

    /**
     * @notice Computes the CREATE2 address of the proxy contract.
     */
    function addr(Proxy memory p) internal pure returns (address) {
        return p.deployment.addr();
    }

    /**
     * @notice Returns a UUPSUpgradeable interface to the proxy contract.
     */
    function uups(Proxy memory p) internal pure returns (UUPSUpgradeable) {
        return UUPSUpgradeable(p.addr());
    }

    /**
     * @notice Performs an idempotent CREATE2 deployment of the proxy contract.
     */
    function idempotentCreate2(Proxy memory p) internal returns (address) {
        return p.deployment.idempotentCreate2();
    }

    /**
     * @notice Creates a proxy struct with a given initial implementation.
     * @dev Assumes OZ's `ERC1967Proxy` construction arguments.
     * @param creationCode The creation code of the proxy contract.
     * @param initialImpl The initial implementation of the proxy contract.
     */
    function makeERC1967Proxy(string memory label, bytes memory creationCode, Implementation memory initialImpl)
        internal
        pure
        returns (Proxy memory)
    {
        return Proxy({
            deployment: Create2FactoryWorker.Deployment({
                label: label,
                salt: initialImpl.deployment.salt,
                creationCode: creationCode,
                constructorArgs: abi.encode(initialImpl.addr(), initialImpl.setupCalldata),
                expectedInitCodeHash: 0x00
            })
        });
    }
}

interface UUPSEvents {
    /**
     * @notice Emitted when an ERC1967 proxy is upgraded. We redefine it here so we can verify that the upgrade is
     * performed.
     * @custom:author Taken from
     * https://github.com/OpenZeppelin/openzeppelin-contracts/blob/v4.8.3/contracts/proxy/ERC1967/ERC1967Upgrade.sol
     */
    event Upgraded(address indexed implementation);
}

/**
 * @notice Module that defines and manages broadcast and prank modes for `UUPSMigrations`.
 * @dev This module defines an auto-prank and auto-broadcast mode. All calls made by `UUPSMigrations` will be
 * automatically pranked or broadcasted, respectively.
 */
abstract contract UUPSMigrationsModes is Script {
    /**
     * @notice Defines the state of a broadcast or prank context.
     * @param addr The address executing the prank or broadcast. Undefined if inactive.
     */
    struct State {
        bool active;
        address addr;
    }

    State public autobroadcast;
    State public autoprank;
    State private _prank;
    State private _broadcast;

    /**
     * @notice Starts a forge prank and logs that a prank context was opened.
     */
    function _startPrank(address a) internal {
        _prank = State({active: true, addr: a});
        vm.startPrank(a, a);
    }

    /**
     * @notice Stops an active forge prank;
     */
    function _stopPrank() internal {
        vm.stopPrank();
        delete _prank;
    }

    /**
     * @notice Starts a forge broadcast and logs that a broadcast context was opened.
     */
    function _startBroadcast(address a) internal {
        _broadcast = State({active: true, addr: a});
        vm.startBroadcast(a);
    }

    /**
     * @notice Stops an active forge broadcast;
     */
    function _stopBroadcast() internal {
        vm.stopBroadcast();
        delete _broadcast;
    }

    /**
     * @notice Enables the auto-pranking mode.
     */
    function startPrank(address prankster) public {
        require(!autobroadcast.active, "UUPSMigrations: cannot prank while broadcasting");
        require(!autoprank.active, "UUPSMigrations: prank already started");
        autoprank = State({active: true, addr: prankster});
    }

    /**
     * @notice Disables the auto-pranking mode.
     */
    function stopPrank() public {
        delete autoprank;
    }

    /**
     * @notice Pranks as the prankster in auto-pranking mode.
     */
    modifier autopranked() {
        if (autoprank.active) {
            _startPrank(autoprank.addr);
        }
        _;
        if (autoprank.active) {
            _stopPrank();
        }
    }

    /**
     * @notice Converts an active broadcast to an autoprank.
     */
    modifier broadcastToAutoprank() {
        State memory autoprank_ = autoprank;
        State memory broadcast = _broadcast;

        if (broadcast.active) {
            autoprank = _broadcast;
            _stopBroadcast();
        }
        _;
        if (broadcast.active) {
            autoprank = autoprank_;
            _startBroadcast(broadcast.addr);
        }
    }

    /**
     * @notice Enables the auto-broadcasting mode.
     */
    function startBroadcast(address broadcaster) public {
        require(!autobroadcast.active, "UUPSMigrations: broadcast already started");
        require(!autoprank.active, "UUPSMigrations: cannot broadcast while pranking");
        autobroadcast = State({active: true, addr: broadcaster});
    }

    /**
     * @notice Disables the auto-broadcasting mode.
     */

    function stopBroadcast() public {
        delete autobroadcast;
    }

    /**
     * @notice Broadcasts as the broadcaster in auto-broadcasting mode.
     */
    modifier autobroadcasted() {
        if (autobroadcast.active) {
            _startBroadcast(autobroadcast.addr);
        }
        _;
        if (autobroadcast.active) {
            _stopBroadcast();
        }
    }
}

/**
 * @notice Script to deal with UUPS contract deployements and migrations.
 * @dev This script assumes constant contract bytecodes to derive deterministic CREATE2 addresses.
 * It is therefore recommended to opt out of storing the contract metadata hash in the contract bytecode (see also:
 * https://docs.soliditylang.org/en/v0.8.18/metadata.html#encoding-of-the-metadata-hash-in-the-bytecode).
 * In forge this can be achived by adding the following to the `foundry.toml`:
 * ```
 * [profile.default]
 * bytecode_hash = "none"
 * cbor_metadata = false
 * ```
 */
abstract contract UUPSMigrations is UUPSMigrationsModes, UUPSEvents {
    using Address for address;
    using Create2FactoryWorker for Create2FactoryWorker.Deployment;
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    /**
     * @notice Returns the list of implementations used by the current project.
     */
    function impls() public view returns (UpgradesLib.Implementation[] memory) {
        uint256 num = numImpls();
        UpgradesLib.Implementation[] memory impls_ = new UpgradesLib.Implementation[](num);
        for (uint256 i; i < num; i++) {
            impls_[i] = impl(i);
        }
        return impls_;
    }

    /**
     * @notice Returns the implementation at the given index.
     * @dev Intended to be overridden by the inheriting contract. Must return implementations for idx in [0,
     * numImpls()). `impl(0)` will be used to deploy the proxy.
     */
    function impl(uint256 idx) public view virtual returns (UpgradesLib.Implementation memory);

    /**
     * @notice The default initial implementation used to deploy the proxy.
     * @dev This is intended to be returned by `impl(0)` if the user chooses to use it.
     */
    function defaultInitialImpl() public pure returns (UpgradesLib.Implementation memory) {
        return UpgradesLib.Implementation({
            deployment: Create2FactoryWorker.Deployment({
                label: "OnlyUpgradeable",
                // Using a fixed salt here since there is no point in constantly redeploying this contract.
                salt: keccak256("PROOF.OnlyUpgradeable"),
                creationCode: type(OnlyUpgradeable).creationCode,
                constructorArgs: "",
                expectedInitCodeHash: 0x66c6956742192e8ed19f4d9f8ecda24a9fd864d853571c2879901d82509940e5
            }),
            setupCalldata: abi.encodeWithSelector(OnlyUpgradeable.initialize.selector)
        });
    }

    /**
     * @notice The number of implementation contracts (i.e. different versions of the project).
     */
    function numImpls() public view virtual returns (uint256);

    /**
     * @notice Returns the proxy used by the current project.
     * @dev Intended to be overridden by the inheriting contract if users want to use something else than a standard
     * ERC1967 proxy.
     */
    function proxy() public view virtual returns (UpgradesLib.Proxy memory) {
        UpgradesLib.Proxy memory p =
            UpgradesLib.makeERC1967Proxy("ERC1967Proxy", type(ERC1967Proxy).creationCode, impl(0));
        // Expected init hash using the `defaultInitialImpl`.
        p.deployment.expectedInitCodeHash = 0x74b9915ae39a059fec6e540c0f5ebfe9cf2da66b01b8ff22a7ea4cca380505c4;
        return p;
    }

    /**
     * @notice Convenience wrapper to return the proxy address
     */
    function proxyAddr() public view returns (address) {
        return proxy().addr();
    }

    /**
     * @dev Storage slot with the address of the current implementation (see EIP1967).
     */
    bytes32 internal constant IMPLEMENTATION_SLOT = bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1);

    /**
     * @notice Reads the current implementation of a given proxy from its EIP1967-defined storage slot.
     */
    function currentImplementation(address proxy_) public view returns (address) {
        return address(uint160(uint256(vm.load(proxy_, IMPLEMENTATION_SLOT))));
    }

    /**
     * @notice Reads the current implementation of a given proxy from its EIP1967-defined storage slot.
     */
    function currentImplementation(UpgradesLib.Proxy memory proxy_) public view returns (address) {
        return currentImplementation(proxy_.addr());
    }

    /**
     * @notice Reads the current implementation of the project proxy from its EIP1967-defined storage slot.
     */
    function currentImplementation() public view returns (address) {
        return currentImplementation(proxy());
    }

    /**
     * @notice Returns whether the proxy has already been deployed.
     */
    function proxyDeployed() public view returns (bool) {
        return proxyAddr().code.length > 0;
    }

    /**
     * @notice Deploys all implementation and proxy contracts idempotently.
     */
    function idempotentDeployAll() public autobroadcasted autopranked {
        UpgradesLib.Implementation[] memory impls_ = impls();
        for (uint256 i; i < impls_.length; i++) {
            impls_[i].idempotentCreate2();
        }
        proxy().idempotentCreate2();
    }

    /**
     * @notice Upgrades the proxy to the given implementation and initialised/reinitialises/sets up the proxy.
     * @param impl_ The implementation to upgrade to.
     * @param wantImpl The implementation that should be the current implementation after the upgrade.
     */
    function _upgradeToAndVerify(UpgradesLib.Implementation memory impl_, address wantImpl) private {
        UpgradesLib.Proxy memory p = proxy();

        if (impl_.setupCalldata.length > 0) {
            p.uups().upgradeToAndCall(impl_.addr(), impl_.setupCalldata);
        } else {
            p.uups().upgradeTo(impl_.addr());
        }

        require(currentImplementation(p) == wantImpl, "UUPSMigrations: wrong implementation after upgrade");
    }

    /**
     * @notice Finds the index of a given implementation address in the list of implementations.
     * @dev Reverts if the implementation is not found or ambiguous (when the list of implementations contains
     * duplicates).
     */
    function findIdx(address impl_, UpgradesLib.Implementation[] memory impls_) public pure returns (uint256) {
        bool found;
        uint256 idx;
        for (uint256 i = 0; i < impls_.length; i++) {
            if (impls_[i].addr() == impl_) {
                require(
                    !found,
                    string.concat(
                        "UUPSMigrations: ambiguous current implementation idx: impl[",
                        vm.toString(i),
                        "] == impl[",
                        vm.toString(idx),
                        "]"
                    )
                );
                idx = i;
                found = true;
            }
        }
        require(found, "UUPSMigrations: current implementation not found");
        return idx;
    }

    /**
     * @notice Finds the index of the current implementation in the list of implementations.
     */
    function implementationIdx() public view returns (uint256) {
        return findIdx(currentImplementation(proxy()), impls());
    }

    /**
     * @notice Upgrades the proxy from the given implementation to the given implementation by performing incremental
     * `upgradeTo`s.
     */
    function migrate(uint256 from, uint256 to) public autobroadcasted autopranked {
        require(from <= to, "UUPSMigrations: migrate: from > to");
        require(to < numImpls(), "UUPSMigrations: migrate: to >= numImpls()");
        require(currentImplementation(proxy()) == impl(from).addr(), "UUPSMigrations: wrong from implementation idx");

        for (uint256 i = from + 1; i <= to; i++) {
            UpgradesLib.Implementation memory im = impl(i);
            _upgradeToAndVerify(im, im.addr());
        }
    }

    /**
     * @notice Upgrades the proxy to the given implementation. Automatically determines the current implementation
     * index.
     */
    function migrate(uint256 to) public {
        migrate(implementationIdx(), to);
    }

    function migrateToLatest() public {
        migrate(numImpls() - 1);
    }

    /**
     * @notice Simulates an upgrade roundtrip to ensure that the current implementation is upgradeable.
     */
    function simulateUpgrade() public broadcastToAutoprank autopranked {
        UpgradesLib.Proxy memory p = proxy();
        address currentImpl = currentImplementation(p);

        UpgradesLib.Implementation memory dummy = UpgradesLib.Implementation({
            deployment: Create2FactoryWorker.Deployment({
                label: "UUPSDummySimulation",
                salt: 0,
                creationCode: type(UUPSDummy).creationCode,
                constructorArgs: "",
                expectedInitCodeHash: 0x00
            }).autoInitCodeHash(),
            // Rolls back immediately after the upgrade. We're leaving this in here as a safeguard for now to prevent
            // any issues if anyone mistakenly broadcasts this.
            // This should be replaced with a broadcast check once https://github.com/foundry-rs/foundry/issues/2900 is
            // resolved.
            setupCalldata: abi.encodeWithSelector(UUPSUpgradeable.upgradeTo.selector, currentImpl)
        });

        dummy.idempotentCreate2();

        vm.record();
        vm.recordLogs();

        _upgradeToAndVerify(dummy, currentImpl);

        // We do our own event logs parsing here instead of using `vm.expectEmit` because we want to revert with a
        // proper error that can be verified in testing.
        VmSafe.Log[] memory logs = vm.getRecordedLogs();
        require(
            contains(logs, upgradedEventLog(dummy.addr(), p.addr())),
            "UUPSMigrations: simulateUpgrade failed - no Upgraded(dummy) event emitted"
        );
        require(
            contains(logs, upgradedEventLog(currentImpl, p.addr())),
            "UUPSMigrations: simulateUpgrade failed - no Upgraded(original) event emitted"
        );

        // The following is not strictly needed since `UUPSDummy` is completely in our control but we check it anyway
        // to ensure that no unintentional storage slots are not written to that could potentially mess with the state
        // of the proxy.
        (, bytes32[] memory writeSlots) = vm.accesses(p.addr());
        for (uint256 i; i < writeSlots.length; i++) {
            require(writeSlots[i] == IMPLEMENTATION_SLOT, "UUPSMigrations: unexpected write slot");
        }
    }

    /**
     * @notice Generates the forge specific log emitted by an `ERC1967Upgrade.Upgraded` event.
     */
    function upgradedEventLog(address newImplementation, address emitter) internal returns (VmSafe.Log memory) {
        vm.recordLogs();
        emit Upgraded(newImplementation);

        VmSafe.Log[] memory logs = vm.getRecordedLogs();
        require(logs.length == 1, "UUPSMigrations: unexpected number of logs");

        logs[0].emitter = emitter;
        return logs[0];
    }

    /**
     * @notice Checks if the given log is contained in the given list of logs.
     */
    function contains(VmSafe.Log[] memory logs, VmSafe.Log memory want) internal pure returns (bool) {
        bytes32[] memory logHashes = new bytes32[](logs.length);
        for (uint256 i; i < logs.length; i++) {
            logHashes[i] = keccak256(abi.encode(logs[i]));
        }

        bytes32 wantHash = keccak256(abi.encode(want));
        for (uint256 i = 0; i < logHashes.length; i++) {
            if (logHashes[i] == wantHash) {
                return true;
            }
        }
        return false;
    }
}
