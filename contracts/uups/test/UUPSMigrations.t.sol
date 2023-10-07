// SPDX-License-Identifier: MIT
// Copyright 2023 PROOF Holdings Inc
pragma solidity ^0.8.15;

import {console2} from "forge-std/console2.sol";
import {VmSafe} from "forge-std/Vm.sol";

import {IERC721, ProofTest, SignerTest, ERC721Fake} from "proof/constants/Testing.sol";

import {ERC1967Proxy} from "openzeppelin-contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {Strings} from "openzeppelin-contracts/utils/Strings.sol";

import {UUPSDummy} from "../src/UUPSDummy.sol";
import {Create2FactoryWorker, UpgradesLib, UUPSMigrations, UUPSEvents} from "../src/UUPSMigrations.s.sol";

interface MockEvents {
    event ImplementationCreated();
    event ProxyCreated();

    event TestableUpgradedTo(address indexed newImplementation);
    event TestableUpgradedToAndCall(
        address indexed newImplementation,
        bytes data
    );
}

contract TestableUUPSImplementation is UUPSDummy, MockEvents {
    constructor() {
        emit ImplementationCreated();
    }

    function upgradeTo(address newImplementation) public virtual override {
        super.upgradeTo(newImplementation);
        emit TestableUpgradedTo(newImplementation);
    }

    function upgradeToAndCall(
        address newImplementation,
        bytes memory data
    ) public payable virtual override {
        super.upgradeToAndCall(newImplementation, data);
        emit TestableUpgradedToAndCall(newImplementation, data);
    }

    function getImplementation() public view returns (address) {
        return _getImplementation();
    }
}

contract TestableUUPSProxy is ERC1967Proxy, MockEvents {
    // Added this so forge does not confuse it with another contract in it's logs (they had the same bytecode before).
    uint256 public constant SOME_CONSTANT = 123;

    constructor(address impl, bytes memory data) ERC1967Proxy(impl, data) {
        emit ProxyCreated();
    }
}

contract ImplWithInitializer is TestableUUPSImplementation {
    /**
     * @notice Emits a `Initialized(1)` event.
     */
    function initialize() public initializer {}
}

contract ImplWithReinitializer is TestableUUPSImplementation {
    uint8 private immutable _reInitVersion;

    constructor(uint8 reInitVersion) {
        _reInitVersion = reInitVersion;
    }

    /**
     * @notice Emits a `Initialized(_reInitVersion)` event.
     */
    function reinitialize() public reinitializer(_reInitVersion) {}
}

contract ImplWithoutUpgradeability is TestableUUPSImplementation {
    function upgradeToAndCall(
        address newImplementation,
        bytes memory data
    ) public payable virtual override {}
}

contract AppendableMigrations is UUPSMigrations {
    using Create2FactoryWorker for Create2FactoryWorker.Deployment;
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    bytes32 public constant DEPLOYMENT_SALT = keccak256("asdf");

    UpgradesLib.Implementation[] private _impls;

    function pushImpl(UpgradesLib.Implementation memory impl_) public {
        _impls.push(impl_);
    }

    function numImpls() public view virtual override returns (uint256) {
        return _impls.length;
    }

    function impl(
        uint256 idx
    ) public view virtual override returns (UpgradesLib.Implementation memory) {
        return _impls[idx];
    }

    function proxy()
        public
        view
        virtual
        override
        returns (UpgradesLib.Proxy memory)
    {
        UpgradesLib.Proxy memory p = UpgradesLib.makeERC1967Proxy(
            "TestableUUPSProxy",
            type(TestableUUPSProxy).creationCode,
            impl(0)
        );
        p.deployment.autoInitCodeHash();
        return p;
    }
}

contract MigrationsTest is ProofTest, MockEvents, UUPSEvents {
    using Create2FactoryWorker for Create2FactoryWorker.Deployment;

    AppendableMigrations migrations;

    bytes32 public constant DEPLOYMENT_SALT = keccak256("asdf");
    uint8 public constant REINIT_VERSION = 2;

    UpgradesLib.Implementation public implWithInitializer;
    UpgradesLib.Implementation public implWithReinitializer;
    UpgradesLib.Implementation public implWithoutInit;
    UpgradesLib.Implementation public implWithoutUpgradeability;

    function makeImplWithReinitializer(
        uint8 version
    ) public pure returns (UpgradesLib.Implementation memory) {
        return
            UpgradesLib.Implementation({
                deployment: Create2FactoryWorker
                    .Deployment({
                        label: "ImplWithReinitializer",
                        salt: DEPLOYMENT_SALT,
                        creationCode: type(ImplWithReinitializer).creationCode,
                        constructorArgs: abi.encode(version),
                        expectedInitCodeHash: 0x00
                    })
                    .autoInitCodeHash(),
                setupCalldata: abi.encodeWithSelector(
                    ImplWithReinitializer.reinitialize.selector
                )
            });
    }

    function setUp() public virtual {
        implWithInitializer = UpgradesLib.Implementation({
            deployment: Create2FactoryWorker
                .Deployment({
                    label: "ImplWithInitializer",
                    salt: DEPLOYMENT_SALT,
                    creationCode: type(ImplWithInitializer).creationCode,
                    constructorArgs: "",
                    expectedInitCodeHash: 0x00
                })
                .autoInitCodeHash(),
            setupCalldata: abi.encodeWithSelector(
                ImplWithInitializer.initialize.selector
            )
        });

        implWithReinitializer = makeImplWithReinitializer(REINIT_VERSION);
        implWithoutInit = UpgradesLib.Implementation({
            deployment: Create2FactoryWorker
                .Deployment({
                    label: "TestableUUPSImplementation",
                    salt: DEPLOYMENT_SALT,
                    creationCode: type(TestableUUPSImplementation).creationCode,
                    constructorArgs: "",
                    expectedInitCodeHash: 0x00
                })
                .autoInitCodeHash(),
            setupCalldata: ""
        });

        implWithoutUpgradeability = UpgradesLib.Implementation({
            deployment: Create2FactoryWorker
                .Deployment({
                    label: "ImplWithoutUpgradeability",
                    salt: DEPLOYMENT_SALT,
                    creationCode: type(ImplWithoutUpgradeability).creationCode,
                    constructorArgs: "",
                    expectedInitCodeHash: 0x00
                })
                .autoInitCodeHash(),
            setupCalldata: ""
        });

        migrations = new AppendableMigrations();
    }
}

contract WrongExpectedInitCodeHashTest is MigrationsTest {
    using Create2FactoryWorker for Create2FactoryWorker.Deployment;
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    UpgradesLib.Implementation public implWithWrongExpectedHash;

    bytes public wrongHashErr;

    function setUp() public virtual override {
        super.setUp();
        implWithWrongExpectedHash = makeImplWithReinitializer(137);
        implWithWrongExpectedHash.deployment.expectedInitCodeHash = bytes32(
            uint256(0xdead)
        );
        wrongHashErr = bytes(
            string.concat(
                "Create2FactoryWorker: unexpected init code hash for ",
                implWithWrongExpectedHash.deployment.label,
                ": actual ",
                Strings.toHexString(
                    uint256(
                        implWithWrongExpectedHash.deployment.initCodeHash()
                    ),
                    32
                ),
                ", expected ",
                Strings.toHexString(
                    uint256(
                        implWithWrongExpectedHash
                            .deployment
                            .expectedInitCodeHash
                    ),
                    32
                )
            )
        );
    }

    function testRevertsOnDeployment() public {
        migrations.pushImpl(implWithWrongExpectedHash);
        vm.expectRevert(wrongHashErr);
        migrations.idempotentDeployAll();
    }

    function testRevertsOnERC1967ProxySetup() public {
        vm.expectRevert(wrongHashErr);
        UpgradesLib.makeERC1967Proxy(
            "TestableUUPSProxy",
            type(TestableUUPSProxy).creationCode,
            implWithWrongExpectedHash
        );
    }

    function testRevertsOnMigrate() public {
        migrations.pushImpl(implWithInitializer);
        migrations.idempotentDeployAll();

        migrations.pushImpl(implWithWrongExpectedHash);
        vm.expectRevert(wrongHashErr);
        migrations.migrateToLatest();
    }
}

contract IdempotencyTest is MigrationsTest {
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    function setUp() public virtual override {
        super.setUp();
        migrations.pushImpl(implWithInitializer);
    }

    /**
     * @dev Triggered when the contract has been initialized or reinitialized.
     */
    event Initialized(uint8 version);

    function testIdempotentDeployAll() public {
        {
            vm.expectEmit(true, true, true, true, migrations.impls()[0].addr());
            emit ImplementationCreated();

            vm.expectEmit(true, true, true, true, migrations.proxy().addr());
            emit Initialized(1);

            vm.expectEmit(true, true, true, true, migrations.proxy().addr());
            emit ProxyCreated();

            migrations.idempotentDeployAll();
        }
        {
            vm.recordLogs();
            migrations.idempotentDeployAll();
            VmSafe.Log[] memory logs = vm.getRecordedLogs();
            assertEq(
                logs.length,
                0,
                "repeated idempotentDeployAll 1: logs recorded"
            );
        }
        {
            migrations.pushImpl(implWithReinitializer);

            vm.expectEmit(true, true, true, true, implWithReinitializer.addr());
            emit ImplementationCreated();

            migrations.idempotentDeployAll();
        }
        {
            vm.recordLogs();
            migrations.idempotentDeployAll();
            VmSafe.Log[] memory logs = vm.getRecordedLogs();
            assertEq(
                logs.length,
                0,
                "repeated idempotentDeployAll 2: logs recorded"
            );
        }
    }

    function testMigrateToLatest() public {
        migrations.idempotentDeployAll();

        {
            vm.recordLogs();
            migrations.migrateToLatest();
            VmSafe.Log[] memory logs = vm.getRecordedLogs();
            assertEq(
                logs.length,
                0,
                "migrateToLatest after deployment: logs recorded"
            );
        }

        UpgradesLib.Implementation[3] memory newImpls;
        for (uint256 i; i < newImpls.length; i++) {
            newImpls[i] = makeImplWithReinitializer(uint8(i + 2));
            migrations.pushImpl(newImpls[i]);
        }
        migrations.idempotentDeployAll();

        for (uint256 i; i < newImpls.length; i++) {
            vm.expectEmit(true, true, true, true, migrations.proxy().addr());
            emit TestableUpgradedToAndCall(
                newImpls[i].addr(),
                newImpls[i].setupCalldata
            );
        }
        migrations.migrateToLatest();

        {
            vm.recordLogs();
            migrations.migrateToLatest();
            VmSafe.Log[] memory logs = vm.getRecordedLogs();
            assertEq(
                logs.length,
                0,
                "migrateToLatest after deployment: logs recorded"
            );
        }
    }
}

contract SimulateUpgradeTest is MigrationsTest {
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    function setUp() public virtual override {
        super.setUp();
        migrations.pushImpl(implWithInitializer);
    }

    function _test(bytes memory err) internal {
        migrations.idempotentDeployAll();
        migrations.migrateToLatest();

        TestableUUPSImplementation p = TestableUUPSImplementation(
            migrations.proxy().addr()
        );
        address currentImpl = p.getImplementation();

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        }
        migrations.simulateUpgrade();

        assertEq(
            p.getImplementation(),
            currentImpl,
            "Implementation changed after simulateUpgrade"
        );
    }

    function testHappy() public {
        _test("");
    }

    function testBrokenUpgrade() public {
        migrations.pushImpl(implWithoutUpgradeability);
        _test(
            bytes(
                "UUPSMigrations: simulateUpgrade failed - no Upgraded(dummy) event emitted"
            )
        );
    }
}

contract MigrateTest is MigrationsTest {
    using UpgradesLib for UpgradesLib.Implementation;
    using UpgradesLib for UpgradesLib.Proxy;

    uint256 private constant MIGRATE_TO_LATEST = type(uint256).max;

    function setUp() public virtual override {
        super.setUp();
        migrations.pushImpl(implWithInitializer);
        migrations.pushImpl(implWithReinitializer);
        migrations.pushImpl(implWithoutInit);
    }

    function _migrate(
        uint256 expectedFrom,
        uint256 migrateTo,
        bytes memory err
    ) internal {
        migrations.idempotentDeployAll();
        UpgradesLib.Implementation[] memory impls = migrations.impls();
        address proxyAddr = migrations.proxy().addr();

        uint256 to = migrateTo == MIGRATE_TO_LATEST
            ? impls.length - 1
            : migrateTo;

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        } else {
            for (uint256 i = expectedFrom + 1; i <= to; i++) {
                vm.expectEmit(true, true, true, true, proxyAddr);
                emit Upgraded(impls[i].addr());
            }
        }
        if (migrateTo == MIGRATE_TO_LATEST) {
            migrations.migrateToLatest();
        } else {
            migrations.migrate(migrateTo);
        }

        assertEq(
            migrations.currentImplementation(),
            impls[fails ? expectedFrom : to].addr(),
            "Wrong implementationIdx after migrateTo"
        );
    }

    function testHappy() public {
        _migrate(0, 1, "");
        _migrate(1, 2, "");
    }

    function testHappyAllInOne() public {
        _migrate(0, 2, "");
    }

    function testToLatestHappy() public {
        _migrate(0, MIGRATE_TO_LATEST, "");
    }

    function testRevertOnAmbiguousImpl() public {
        migrations.pushImpl(implWithReinitializer); // This was already pushed in `setUp()`
        _migrate(0, 1, "");
        _migrate(
            1,
            2,
            "UUPSMigrations: ambiguous current implementation idx: impl[3] == impl[1]"
        );
    }
}
