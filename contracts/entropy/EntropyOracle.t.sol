// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.19;

import "./EntropyOracle.sol";
import {IEntropyOracleEvents} from "./IEntropyOracle.sol";
import {Test} from "ethier_root/tests/TestLib.sol";
import {Strings} from "openzeppelin-contracts/utils/Strings.sol";
import {Vm} from "forge-std/Vm.sol";

contract EntropyOracleTest is Test, IEntropyOracleEvents {
    using Strings for uint256;

    EntropyOracle public impl;

    /**
     * @notice Identical to `impl` but only providing access to interface-defined functions.
     */
    IEntropyOracle public oracle;

    address public admin = makeAddr("admin");

    address public steerer = makeAddr("steerer");

    address public requester = makeAddr("requester");

    address public vandal = makeAddr("vandal");

    function _setUp() internal virtual {
        impl = new EntropyOracle(admin, steerer);
        oracle = impl;
    }

    function _assertNoBlockEntropy(uint256 blockNumber) internal {
        assertEq(
            oracle.blockEntropy(blockNumber),
            0,
            string.concat("blockEntropy(", blockNumber.toString(), ") expected to be null")
        );
    }

    function _assertBlockEntropyFromSignature(uint256 blockNumber, bytes memory sig) internal {
        assertEq(
            oracle.blockEntropy(blockNumber),
            keccak256(sig),
            string.concat("blockEntropy(", blockNumber.toString(), ") MUST be keccak256 of block signature")
        );
    }
}

contract VandalTest is EntropyOracleTest {
    function setUp() public virtual {
        _setUp();
    }

    function testCannotRequestEntropy() public {
        vm.expectRevert(missingRoleError(vandal, impl.ENTROPY_REQUESTER_ROLE()));
        vm.prank(vandal, steerer);
        oracle.requestEntropy();
    }

    function testCannotRequestEntropy(uint256 blockNumber) public {
        vm.expectRevert(missingRoleError(vandal, impl.ENTROPY_REQUESTER_ROLE()));
        vm.prank(vandal, steerer);
        oracle.requestEntropy(blockNumber);
    }

    function testCannotSetSigner(address eve) public {
        vm.expectRevert(missingRoleError(vandal, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        impl.setSigner(eve);
    }
}

contract EntropyRequesterRoleAssignmentTest is EntropyOracleTest {
    bytes32 public requesterRole;

    function setUp() public virtual {
        _setUp();
        requesterRole = impl.ENTROPY_REQUESTER_ROLE();
    }

    function testSteererGrantRequesterRole(address newRequester) public {
        vm.prank(steerer);
        impl.grantRole(requesterRole, newRequester);
    }

    function testAdminCannotGrantRequesterRole(address newRequester) public {
        vm.expectRevert(missingRoleError(admin, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(admin);
        impl.grantRole(requesterRole, newRequester);
    }
}

contract RequestTest is EntropyOracleTest {
    function setUp() public virtual {
        _setUp();

        vm.startPrank(steerer);
        impl.grantRole(impl.ENTROPY_REQUESTER_ROLE(), requester);
        vm.stopPrank();

        vm.startPrank(requester);
    }

    struct TestCase {
        uint256 currBlock;
        uint256 requestBlock;
    }

    function testExplicitRequestArbitraryBlock(TestCase memory tt) public {
        // See the @dev security warning regarding why we don't assume that
        // requestBlock is in the future.
        _testRequestExplicitBlock(tt);
    }

    function testExplicitRequestCurrentBlock(uint256 currBlock) public {
        _testRequestExplicitBlock(TestCase({currBlock: currBlock, requestBlock: currBlock}));
    }

    function _testRequestExplicitBlock(TestCase memory tt) internal {
        vm.roll(tt.currBlock);

        vm.expectEmit(true, true, true, true, address(oracle));
        emit EntropyRequested(tt.requestBlock);
        oracle.requestEntropy(tt.requestBlock);

        // Idempotency avoids an automated task attempting multiple fulfilments.
        vm.recordLogs();
        oracle.requestEntropy(tt.requestBlock);
        Vm.Log[] memory logs = vm.getRecordedLogs();
        assertEq(logs.length, 0);

        // The idempotency is achieved by setting block entropy to 1 as a sentinel. Ensure that this isn't exposed to
        // the end user.
        assertEq(oracle.blockEntropy(tt.requestBlock), 0);
    }

    function testImplicitRequestCurrentBlock(uint256 currBlock) public {
        vm.roll(currBlock);

        vm.expectEmit(true, true, true, true, address(oracle));
        emit EntropyRequested(currBlock);
        oracle.requestEntropy();
    }
}

contract EntropyTest is EntropyOracleTest {
    function setUp() public virtual {
        _setUp();
    }

    struct TestCase {
        uint64 currBlock;
        uint256 privateKey;
        uint64 signBlock;
        uint64 provideBlock;
        address provider;
    }

    function testCanProvideEntropy(TestCase memory tt) public {
        vm.assume(tt.provideBlock < tt.currBlock);
        tt.signBlock = tt.provideBlock;
        _testProvideEntropy(tt, "");
    }

    function testCannotProvideFutureEntropy(TestCase memory tt) public {
        vm.assume(tt.provideBlock >= tt.currBlock);
        _testProvideEntropy(tt, abi.encodeWithSelector(EntropyOracle.NonHistoricalBlock.selector, tt.provideBlock));
    }

    function testCannotProvideInvalidEntropy(TestCase memory tt) public {
        vm.assume(tt.provideBlock < tt.currBlock);
        vm.assume(tt.provideBlock != tt.signBlock);
        _testProvideEntropy(tt, abi.encodeWithSelector(EntropyOracle.InvalidEntropySignature.selector));
    }

    function _testProvideEntropy(TestCase memory tt, bytes memory err) internal {
        vm.roll(tt.currBlock);
        _assertNoBlockEntropy(tt.provideBlock);

        vm.prank(steerer);
        impl.setSigner(_signer(tt.privateKey));

        // Anyone is allowed to provide the entropy as it's gated by signature.
        vm.assume(tt.provider != address(0));
        vm.startPrank(tt.provider);

        bytes memory sig = _signBlock(tt.privateKey, tt.signBlock);

        bool fail = err.length > 0;
        if (fail) {
            vm.expectRevert(err);
        } else {
            vm.expectEmit(true, true, true, true, address(oracle));
            emit EntropyProvided(tt.provideBlock, keccak256(sig));
        }
        impl.provideEntropy(EntropyOracle.EntropyFulfilment({blockNumber: tt.provideBlock, signature: sig}));

        if (fail) {
            _assertNoBlockEntropy(tt.provideBlock);
            return;
        }

        _assertBlockEntropyFromSignature(tt.provideBlock, sig);

        vm.expectRevert(abi.encodeWithSelector(EntropyOracle.EntropyAlreadyProvided.selector, tt.provideBlock));
        impl.provideEntropy(EntropyOracle.EntropyFulfilment({blockNumber: tt.provideBlock, signature: sig}));
    }

    function testProvideMultipleBlocks(
        uint256 privateKey,
        uint128 startBlock,
        uint128[] memory blockDeltas,
        address provider
    ) public {
        vm.prank(steerer);
        impl.setSigner(_signer(privateKey));

        vm.assume(blockDeltas.length < 10); // faster tests
        EntropyOracle.EntropyFulfilment[] memory entropy = new EntropyOracle.EntropyFulfilment[](
                blockDeltas.length
            );

        // We have to have unique block numbers otherwise an EntropyAlreadyProvided error will be thrown. As we can't
        // request this from the fuzzer, we instead accept deltas and a starting point from which we derive all block
        // numbers. The final delta determines the current block as this must also be ahead of the provided entropy.
        uint256 blockNumber = startBlock;
        for (uint256 i = 0; i < blockDeltas.length; ++i) {
            _assertNoBlockEntropy(blockNumber);

            entropy[i].blockNumber = blockNumber;
            entropy[i].signature = _signBlock(privateKey, blockNumber);

            vm.expectEmit(true, true, true, true, address(oracle));
            emit EntropyProvided(blockNumber, keccak256(entropy[i].signature));

            blockNumber += uint256(blockDeltas[i]) + 1; // +1 avoids zero block delta / EntropyAlreadyProvided errors
        }
        vm.roll(blockNumber);

        vm.prank(provider);
        impl.provideEntropy(entropy);

        for (uint256 i = 0; i < blockDeltas.length; ++i) {
            _assertBlockEntropyFromSignature(entropy[i].blockNumber, entropy[i].signature);
        }
    }

    function _setEntropy(uint256 privateKey, uint256 blockNumber) internal returns (bytes32) {
        vm.assume(blockNumber < 1 << 255);
        vm.roll(blockNumber + 1);

        vm.prank(steerer);
        impl.setSigner(_signer(privateKey));
        _signer(privateKey);

        bytes memory sig = _signBlock(privateKey, blockNumber);
        impl.provideEntropy(EntropyOracle.EntropyFulfilment({blockNumber: blockNumber, signature: sig}));

        return keccak256(sig);
    }

    function _signer(uint256 privateKey) internal pure returns (address) {
        vm.assume(privateKey > 0);
        vm.assume(
            privateKey < 115792089237316195423570985008687907852837564279074904382605163141518161494337 // curve order
        );
        return vm.addr(privateKey);
    }

    function _signBlock(uint256 pk, uint256 blockNumber) internal view returns (bytes memory) {
        return _sign(pk, blockDigest(blockNumber));
    }

    function _sign(uint256 pk, bytes32 digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pk, digest);
        return abi.encodePacked(r, s, v);
    }
}
