// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.

pragma solidity >=0.8.0 <0.9.0;

import {ProofTest} from "proof/constants/Testing.sol";
import {PurchaseByProjectIDLib} from "proof/sellers/sellable/SellableERC721ACommonByProjectID.sol";

import {ProjectPoolSellable} from "../src/pool/ProjectPoolSellable.sol";

contract TestableProjectPoolSellable is ProjectPoolSellable {
    constructor(Init memory init) ProjectPoolSellable(init) {}

    uint128 public numProjects;

    mapping(uint128 => uint64) public maxNumPerProject;

    function setNumProjects(uint128 numProjects_) public {
        numProjects = numProjects_;
    }

    function setMaxNumPerProject(uint128 projectId, uint64 maxNumPerProject_) public {
        maxNumPerProject[projectId] = maxNumPerProject_;
    }

    function _numProjects() internal view virtual override returns (uint128) {
        return numProjects;
    }

    function _maxNumPerProject(uint128 projectId) internal view virtual override returns (uint64) {
        return maxNumPerProject[projectId];
    }

    function burn(uint256 tokenId) public {
        _burn(tokenId, true);
    }

    function nextTokenId() public view returns (uint256) {
        return _nextTokenId();
    }
}

contract ProjectPoolSellableTest is ProofTest {
    address public immutable seller = makeAddr("seller");

    TestableProjectPoolSellable public pool;

    function setupPool() public virtual returns (TestableProjectPoolSellable) {
        return new TestableProjectPoolSellable(ProjectPoolSellable.Init({
            admin: admin,
            steerer: steerer,
            name: "name",
            symbol: "symbol",
            royaltyReciever: payable(address(0xFEE)),
            royaltyBasisPoints: 100,
            baseURI: "baseURI"
        }));
    }

    function setUp() public virtual {
        pool = setupPool();

        vm.startPrank(steerer);
        pool.grantRole(pool.AUTHORISED_SELLER_ROLE(), seller);
        vm.stopPrank();
    }

    function _purchase(address to, uint128[] memory projectIds, bytes memory err)
        internal
        assertERC721BalanceChanged(
            ERC721BalanceDelta({token: address(pool), account: to, delta: zeroIfErrElse(err, int256(projectIds.length))})
        )
        assertERC721TotalSupplyChangedBy(address(pool), zeroIfErrElse(err, int256(projectIds.length)))
    {
        uint256 startTokenId = pool.nextTokenId();

        bool fails = err.length > 0;
        if (fails) {
            vm.expectRevert(err);
        }
        vm.prank(seller);
        pool.handleSale(to, uint64(projectIds.length), PurchaseByProjectIDLib.encodePurchaseData(projectIds));

        if (fails) {
            return;
        }

        for (uint256 i; i < projectIds.length; ++i) {
            uint256 tokenId = startTokenId + i;
            assertEq(pool.tokenInfo(tokenId).projectId, projectIds[i]);
        }
    }
}

contract PurchaseTest is ProjectPoolSellableTest {
    uint128 constant NUM_PROJECTS = 5;

    function setUp() public virtual override {
        super.setUp();

        pool.setNumProjects(NUM_PROJECTS);
        for (uint64 i; i < NUM_PROJECTS; ++i) {
            pool.setMaxNumPerProject(i, 2);
        }
    }

    function testSuccess(address to) public {
        _purchase(to, toUint128s([1, 2, 4]), "");
        _purchase(to, toUint128s([3, 1]), "");

        uint64[] memory nums = pool.numPurchasedPerProject();
        assertEq(nums.length, NUM_PROJECTS);
        assertEq(nums[0], 0);
        assertEq(nums[1], 2);
        assertEq(nums[2], 1);
        assertEq(nums[3], 1);
        assertEq(nums[4], 1);

        assertEq(new uint256[](0), pool.tokenIdsByProjectId(0));
        assertEq(toUint256s([0, 4]), pool.tokenIdsByProjectId(1));
        assertEq(toUint256s([1]), pool.tokenIdsByProjectId(2));
        assertEq(toUint256s([3]), pool.tokenIdsByProjectId(3));
        assertEq(toUint256s([2]), pool.tokenIdsByProjectId(4));
    }

    function testExceedingProjectSize(address to) public {
        _purchase(
            to, toUint128s([1, 2, 1, 1]), abi.encodeWithSelector(ProjectPoolSellable.ProjectExhausted.selector, 1)
        );
    }

    function testExceedingProjectSizeSequentially(address to) public {
        _purchase(to, toUint128s([1, 1]), "");
        _purchase(to, toUint128s([2, 1]), abi.encodeWithSelector(ProjectPoolSellable.ProjectExhausted.selector, 1));

        pool.setMaxNumPerProject(1, 3);
        _purchase(to, toUint128s([2, 1]), "");
    }

    function testExceedingProjectSizeFuzzed(address to, uint128 projectId, uint64 numMax) public {
        numMax = numMax % 500;
        vm.assume(numMax > 0);
        vm.assume(projectId < type(uint128).max);

        pool.setNumProjects(projectId + 1);
        pool.setMaxNumPerProject(projectId, numMax);

        uint128[] memory projectIds = new uint128[](numMax);
        for (uint256 i; i < numMax; ++i) {
            projectIds[i] = projectId;
        }
        _purchase(to, projectIds, "");

        _purchase(
            to,
            toUint128s([projectId]),
            abi.encodeWithSelector(ProjectPoolSellable.ProjectExhausted.selector, projectId)
        );
    }

    function testInvalidProjectId(address to, uint128 projectId) public {
        vm.assume(projectId >= pool.numProjects());
        _purchase(
            to,
            toUint128s([1, projectId]),
            abi.encodeWithSelector(ProjectPoolSellable.InvalidProject.selector, projectId)
        );
    }
}

/**
 * @notice While the burn functionality is not explicitly exposed/modified in the contract, the base contract still
 * allows it. So we want to make sure that it does not interfere with any functionality.
 */
contract BurnTest is ProjectPoolSellableTest {
    uint128 constant NUM_PROJECTS = 5;

    function setUp() public virtual override {
        super.setUp();

        pool.setNumProjects(NUM_PROJECTS);
        for (uint64 i; i < NUM_PROJECTS; ++i) {
            pool.setMaxNumPerProject(i, 2);
        }
    }

    function testGettersUnaffected(address to, uint128 projectId1, uint128 projectId2) public {
        vm.assume(projectId1 < pool.numProjects());
        vm.assume(projectId2 < pool.numProjects());

        _purchase(to, toUint128s([projectId1]), "");

        uint64[] memory nums = pool.numPurchasedPerProject();
        assertEq(nums[projectId1], 1);

        uint256[] memory project1Tokens = pool.tokenIdsByProjectId(projectId1);
        assertEq(project1Tokens.length, 1);
        assertEq(project1Tokens[0], 0);

        vm.prank(to);
        pool.burn(0);

        uint64[] memory numsAfterBurn = pool.numPurchasedPerProject();
        assertEq(numsAfterBurn, nums);

        uint256[] memory project1TokensAfterBurn = pool.tokenIdsByProjectId(projectId1);
        assertEq(project1TokensAfterBurn, project1Tokens);

        _purchase(to, toUint128s([projectId2]), "");

        nums[projectId2]++;
        uint64[] memory numsAfterMint = pool.numPurchasedPerProject();
        assertEq(numsAfterMint, nums);

        assertEq(pool.tokenIdsByProjectId(projectId1).length, nums[projectId1]);
        assertEq(pool.tokenIdsByProjectId(projectId2).length, nums[projectId2]);
    }

    function testPurchaseLimitsUnchanged(address to, uint128 projectId, uint64 numMax, uint256 burnTokenId) public {
        numMax = numMax % 500;
        vm.assume(numMax > 0);
        vm.assume(projectId < type(uint128).max);

        pool.setNumProjects(projectId + 1);
        pool.setMaxNumPerProject(projectId, numMax);

        uint128[] memory projectIds = new uint128[](numMax);
        for (uint256 i; i < numMax; ++i) {
            projectIds[i] = projectId;
        }
        _purchase(to, projectIds, "");

        vm.prank(to);
        pool.burn(burnTokenId % numMax);

        _purchase(
            to,
            toUint128s([projectId]),
            abi.encodeWithSelector(ProjectPoolSellable.ProjectExhausted.selector, projectId)
        );
    }
}
