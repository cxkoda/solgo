// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.19;

import "./EntropyOracleV2.sol";

import {IEntropyOracleV2, IEntropyOracleV2Events, IEntropyConsumer} from "./IEntropyOracleV2.sol";
import {IEntropyOracle as IEntropyOracleV1} from "./IEntropyOracle.sol";
import {Test} from "ethier_root/tests/TestLib.sol";

import {
    EntropyOracleTest as EntropyOracleV1Test,
    VandalTest as V1VandalTest,
    EntropyRequesterRoleAssignmentTest as V1EntropyRequesterRoleAssignmentTest,
    RequestTest as V1RequestTest,
    EntropyTest as V1EntropyTest
} from "./EntropyOracle.t.sol";

contract EntropyOracleV2Test is EntropyOracleV1Test {
    EntropyOracleV2 public implV2;
    IEntropyOracleV2 public oracleV2;

    function _setUp() internal virtual override {
        implV2 = new EntropyOracleV2(admin, steerer);
        impl = implV2;
        oracle = implV2;
        oracleV2 = implV2;
    }
}

contract VandalTest is EntropyOracleV2Test, V1VandalTest {
    function _setUp() internal virtual override(EntropyOracleV1Test, EntropyOracleV2Test) {
        EntropyOracleV2Test._setUp();
    }

    function testCannotSetOverride(address v1) public {
        vm.expectRevert(missingRoleError(vandal, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        implV2.setOverride(EntropyOracle(v1));
    }

    function testCannotRemoveConsumerCallback(uint96 callbackId, uint256 blockNumber, address consumer) public {
        vm.expectRevert(missingRoleError(vandal, impl.DEFAULT_STEERING_ROLE()));
        vm.prank(vandal, steerer);
        implV2.removeCallback(consumer, callbackId, blockNumber);
    }
}

contract EntropyRequesterRoleAssignmentTest is EntropyOracleV2Test, V1EntropyRequesterRoleAssignmentTest {
    function _setUp() internal virtual override(EntropyOracleV1Test, EntropyOracleV2Test) {
        EntropyOracleV2Test._setUp();
    }
}

contract RequestTest is EntropyOracleV2Test, V1RequestTest {
    function _setUp() internal virtual override(EntropyOracleV1Test, EntropyOracleV2Test) {
        EntropyOracleV2Test._setUp();
    }
}

interface CallbackEvent {
    event Callback(uint256 indexed blockNumber, uint96 indexed callbackId, bytes32 indexed entropy);
}

contract CallbackRequester is IEntropyConsumer, CallbackEvent {
    // If you see this there's a bug in the callback triggering that doesn't gracefully handle callback reverts.
    error ConsumerReverted();

    bool private _reverts = false;

    function callbackReverts() external view returns (bool) {
        return _reverts;
    }

    function setCallbackReverts(bool reverts) external {
        _reverts = reverts;
    }

    error AlreadyConsumed(uint256 blockNumber, uint96 callbackId);

    mapping(uint256 => mapping(uint96 => bool)) private _consumed;

    function consumeEntropy(uint256 blockNumber, uint96 callbackId, bytes32 entropy) public virtual {
        if (_reverts) {
            revert ConsumerReverted();
        }

        if (_consumed[blockNumber][callbackId]) {
            revert AlreadyConsumed(blockNumber, callbackId);
        }
        _consumed[blockNumber][callbackId] = true;

        emit Callback(blockNumber, callbackId, entropy);
    }

    function consumed(uint96 callbackId, uint256 blockNumber) public view returns (bool) {
        return _consumed[blockNumber][callbackId];
    }
}

contract StubV1EntropyOracle is IEntropyOracleV1 {
    mapping(uint256 => bytes32) public blockEntropy;

    function setEntropy(uint256 blockNumber, bytes32 entropy) external {
        blockEntropy[blockNumber] = entropy;
    }

    error Unimplemented();

    function requestEntropy() external pure {
        revert Unimplemented();
    }

    function requestEntropy(uint256) external pure {
        revert Unimplemented();
    }
}

contract Noop {}

contract EntropyTest is EntropyOracleV2Test, V1EntropyTest, CallbackEvent, IEntropyOracleV2Events {
    function _setUp() internal virtual override(EntropyOracleV1Test, EntropyOracleV2Test) {
        EntropyOracleV2Test._setUp();
    }

    CallbackRequester[] public requesters;

    function setUp() public virtual override {
        _setUp();

        vm.startPrank(steerer);
        impl.grantRole(impl.ENTROPY_REQUESTER_ROLE(), requester);
        for (uint256 i = 0; i < 10; ++i) {
            CallbackRequester cbRequester = new CallbackRequester();
            impl.grantRole(impl.ENTROPY_REQUESTER_ROLE(), address(cbRequester));
            requesters.push(cbRequester);

            if (i == 0) {
                cbRequester.setCallbackReverts(true);
            }
        }
        vm.stopPrank();
    }

    function _requestAs(address req) internal {
        vm.prank(req);
        oracleV2.requestEntropyWithCallback();
    }

    function _requestAs(address req, uint96 callbackId, uint256 blockNumber) internal {
        vm.prank(req);
        oracleV2.requestEntropyWithCallback(blockNumber, callbackId);
    }

    function testRequestWithCallback(uint256 privateKey, uint256 currBlock) public {
        vm.assume(currBlock > 0);
        vm.assume(currBlock < 1 << 255);

        vm.prank(steerer);
        impl.setSigner(_signer(privateKey));
        bytes memory sig = _signBlock(privateKey, currBlock);

        vm.expectEmit(true, true, true, true, address(oracle));
        emit EntropyRequested(currBlock);

        vm.roll(currBlock);
        for (uint256 i = 0; i < requesters.length; ++i) {
            CallbackRequester req = requesters[i];

            vm.recordLogs();
            _requestAs(address(req));
            // Only the first request will emit an event (the request). This is tested elsewhere and we're only
            // interested in demonstrating that no others are emitted by our tests, which would cause spurious
            // failure of expectEmit()s.
            assertEq(vm.getRecordedLogs().length, i == 0 ? 1 : 0);
        }

        // Ensure graceful handling if the caller is an EOA.
        _requestAs(requester);

        // Ensure graceful failure if the caller isn't an IEntropyConsumer.
        address noop = address(new Noop());
        vm.startPrank(steerer);
        implV2.grantRole(implV2.ENTROPY_REQUESTER_ROLE(), noop);
        vm.stopPrank();

        vm.prank(noop);
        oracleV2.requestEntropyWithCallback();

        // Events emitted by the oracle or requester on fulfillment/callback
        for (uint256 i = 0; i < requesters.length; ++i) {
            CallbackRequester req = requesters[i];
            if (req.callbackReverts()) {
                vm.expectEmit(true, true, true, true, address(oracleV2));
                emit CallbackFailed(
                    currBlock, address(req), abi.encodeWithSelector(CallbackRequester.ConsumerReverted.selector)
                );
            } else {
                vm.expectEmit(true, true, true, true, address(req));
                emit Callback(currBlock, 0, keccak256(sig));
            }
        }
        vm.expectEmit(true, true, true, true, address(oracleV2));
        emit CallbackFailed(currBlock, noop, "");

        EntropyOracle.EntropyFulfilment[] memory entropy = new EntropyOracle.EntropyFulfilment[](1);
        entropy[0].blockNumber = currBlock;
        entropy[0].signature = sig;

        vm.roll(currBlock + 1);
        implV2.provideEntropy(entropy);

        for (uint256 i = 0; i < requesters.length; ++i) {
            assertEq(
                requesters[i].consumed(0, currBlock),
                !requesters[i].callbackReverts(),
                "entropy consumed i.f.f. non-reverting"
            );
        }

        // Ensures that they're not called again on those that were successful the first time, otherwise AlreadyConsumed
        // will be thrown.
        implV2.triggerCallbacks(currBlock, implV2.numCallbacks(currBlock));
    }

    function testCallbackRequesterReverts(uint96 callbackId, uint96 callbackId2, uint256 blockNumber, bytes32 entropy)
        public
    {
        vm.assume(callbackId != callbackId2);

        // All other tests rely on the lack of the requester reverting as an assertion that consumeEntropy() was called
        // only once. Just making sure it's not a false negative.
        CallbackRequester req = new CallbackRequester();
        req.consumeEntropy(blockNumber, callbackId, entropy);

        // Different ID doesn't revert.
        req.consumeEntropy(blockNumber, callbackId2, entropy);

        vm.expectRevert(abi.encodeWithSelector(CallbackRequester.AlreadyConsumed.selector, blockNumber, callbackId));
        req.consumeEntropy(blockNumber, callbackId, entropy);
    }

    function testRequestWithV1Override(uint256 privateKey, uint256 blockNumber, bytes32 overrideEntropy) public {
        vm.assume(overrideEntropy > 0);

        assertEq(oracleV2.blockEntropy(blockNumber), 0, "no entropy available");

        // No override set so provide "local" entropy.
        bytes32 actualEntropy = _setEntropy(privateKey, blockNumber);
        assertEq(oracleV2.blockEntropy(blockNumber), actualEntropy, "local entropy");

        // Override without entropy still uses "local" entropy.
        StubV1EntropyOracle stub = new StubV1EntropyOracle();
        vm.prank(steerer);
        implV2.setOverride(stub);
        assertEq(oracleV2.blockEntropy(blockNumber), actualEntropy, "override exists but has no entropy");

        // Propagate the override source when non-zero.
        stub.setEntropy(blockNumber, overrideEntropy);
        assertEq(oracleV2.blockEntropy(blockNumber), overrideEntropy, "override entropy preferred");
    }

    function testAutomaticCallbackWhenEntropyAlreadySet(uint256 privateKey, uint96 callbackId, uint256 blockNumber)
        public
    {
        bytes32 entropy = _setEntropy(privateKey, blockNumber);

        CallbackRequester req = requesters[0];
        req.setCallbackReverts(false);

        vm.expectEmit(true, true, true, true, address(req));
        emit Callback(blockNumber, callbackId, entropy);
        _requestAs(address(req), callbackId, blockNumber);
    }

    struct RetryTestCase {
        uint256 privateKey;
        uint96 callbackId;
        uint256 blockNumber;
        bool removeCallback;
    }

    function testCallbackRetry(RetryTestCase memory tt) public {
        CallbackRequester[] memory reqs = new CallbackRequester[](2);

        for (uint256 i = 0; i < reqs.length; ++i) {
            reqs[i] = new CallbackRequester();
            vm.startPrank(steerer);
            implV2.grantRole(implV2.ENTROPY_REQUESTER_ROLE(), address(reqs[i]));
            vm.stopPrank();

            reqs[i].setCallbackReverts(true);
            _requestAs(address(reqs[i]), tt.callbackId, tt.blockNumber);
        }

        _setEntropy(tt.privateKey, tt.blockNumber);
        for (uint256 i = 0; i < reqs.length; ++i) {
            assertFalse(
                reqs[i].consumed(tt.callbackId, tt.blockNumber), "entropy not already consumed because reverted"
            );
            reqs[i].setCallbackReverts(false);
        }

        if (tt.removeCallback) {
            vm.startPrank(steerer);
            implV2.removeCallback(address(reqs[1]), tt.callbackId, tt.blockNumber);

            // Removing again will revert because the callback no longer exists.
            vm.expectRevert(
                abi.encodeWithSelector(
                    EntropyOracleV2.CallbackNotRegistered.selector, address(reqs[1]), tt.callbackId, tt.blockNumber
                )
            );
            implV2.removeCallback(address(reqs[1]), tt.callbackId, tt.blockNumber);
            vm.stopPrank();
        }

        implV2.triggerCallbacks(tt.blockNumber, implV2.numCallbacks(tt.blockNumber));
        assertTrue(reqs[0].consumed(tt.callbackId, tt.blockNumber), "non-reverting consumer consumed entropy");
        assertEq(reqs[1].consumed(tt.callbackId, tt.blockNumber), !tt.removeCallback);
    }

    function testPrematureTrigger(uint256 blockNumber) public {
        vm.expectRevert(abi.encodeWithSelector(EntropyOracleV2.EntropyNotAvailable.selector, blockNumber));
        implV2.triggerCallbacks(blockNumber, 1);
    }

    function testDifferentCallbackIds(uint256 privateKey, uint32[10] memory callbackIdDeltas, uint256 blockNumber)
        public
    {
        CallbackRequester req = requesters[0];
        req.setCallbackReverts(false);
        address reqAddr = address(req);

        uint96[10] memory callbackIds;
        uint96 id;
        for (uint256 i = 0; i < callbackIdDeltas.length; ++i) {
            callbackIds[i] = id;

            assertFalse(implV2.isCallbackRegistered(reqAddr, id, blockNumber), "callback not registered before request");
            _requestAs(reqAddr, id, blockNumber);
            assertTrue(implV2.isCallbackRegistered(reqAddr, id, blockNumber), "callback registered after request");
            assertEq(implV2.numCallbacks(blockNumber), i + 1, "numCallbacks when requesting");

            (address consumer, uint96 callbackId) = implV2.callback(blockNumber, i);
            assertEq(consumer, reqAddr, "introspection of arbitrary callback consumer");
            assertEq(callbackId, id, "introspection of arbitrary callback ID");

            id += uint96(callbackIdDeltas[i]) + 1; // +1 guarantees no duplicte IDs
        }

        uint256 removed;
        for (uint256 i = 0; i < callbackIds.length; ++i) {
            if (i % 3 == 0) {
                vm.prank(steerer);
                implV2.removeCallback(reqAddr, callbackIds[i], blockNumber);
                ++removed;
            }
            assertEq(
                implV2.isCallbackRegistered(reqAddr, callbackIds[i], blockNumber),
                i % 3 != 0,
                "callback registered i.f.f. not removed"
            );
            assertEq(implV2.numCallbacks(blockNumber), callbackIds.length - removed, "numCallbacks after removing");
        }

        _setEntropy(privateKey, blockNumber);

        for (uint256 i = 0; i < callbackIds.length; ++i) {
            assertEq(req.consumed(callbackIds[i], blockNumber), i % 3 != 0);
        }
    }

    function testMaxCallbacks(
        uint256 privateKey,
        uint256 blockNumber,
        uint256 maxProvideCallbacks,
        uint256 maxTriggerCallbacks
    ) public {
        vm.assume(blockNumber > 0);
        vm.assume(blockNumber < 1 << 255);
        for (uint256 i = 0; i < requesters.length; ++i) {
            requesters[i].setCallbackReverts(false);

            uint96 callbackId = uint96(uint256(keccak256(abi.encodePacked(blockNumber, i))));
            _requestAs(address(requesters[i]), callbackId, blockNumber);
        }
        assertEq(implV2.numCallbacks(blockNumber), requesters.length, "all requesters have callbacks");

        address signer = _signer(privateKey);
        vm.prank(steerer);
        implV2.setSigner(signer);

        vm.roll(blockNumber + 1);
        implV2.provideEntropy(
            EntropyOracle.EntropyFulfilment({blockNumber: blockNumber, signature: _signBlock(privateKey, blockNumber)}),
            maxProvideCallbacks
        );

        uint256 remaining = _nonNegativeDiffOrZero(requesters.length, maxProvideCallbacks);
        assertEq(implV2.numCallbacks(blockNumber), remaining, "remaining callbacks after providing entropy");

        implV2.triggerCallbacks(blockNumber, maxTriggerCallbacks);
        assertEq(
            implV2.numCallbacks(blockNumber),
            _nonNegativeDiffOrZero(remaining, maxTriggerCallbacks),
            "remaining callbacks after triggering"
        );
    }

    /**
     * @return max(a-b, 0);
     */
    function _nonNegativeDiffOrZero(uint256 a, uint256 b) internal pure returns (uint256) {
        return a > b ? a - b : 0;
    }
}

contract CallbackPacker is EntropyOracleV2 {
    constructor() EntropyOracleV2(address(0), address(0)) {}

    function pack(address addr, uint96 id) external pure returns (bytes32) {
        return _packCallback(addr, id);
    }

    function unpack(bytes32 packed) external pure returns (address, uint96) {
        return _unpackCallback(packed);
    }
}

contract PackingTest is Test {
    function testPackingRoundTrip(address addr, uint96 id) public {
        CallbackPacker p = new CallbackPacker();
        (address unpackedAddr, uint96 unpackedId) = p.unpack(p.pack(addr, id));
        assertEq(addr, unpackedAddr);
        assertEq(id, unpackedId);
    }
}

// TODO: test provideEntropy with maxCallbacks less than number registered.
