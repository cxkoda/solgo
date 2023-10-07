// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.

pragma solidity >=0.8.0 <0.9.0;

import {GenArt721CoreV3_Engine_Flex_PROOF} from "artblocks-contracts/GenArt721CoreV3_Engine_Flex_PROOF.sol";

import {ProofTest} from "proof/constants/Testing.sol";
import {PurchaseByProjectIDLib} from "proof/sellers/sellable/SellableERC721ACommonByProjectID.sol";
import {IGenArt721CoreContractV3_Mintable} from "proof/artblocks/IGenArt721CoreContractV3_Mintable.sol";
import {artblocksTokenID} from "proof/artblocks/TokenIDMapping.sol";
import {ArtblocksTest} from "proof/artblocks/ArtblocksTest.t.sol";

import {ABProjectPoolSellable, ProjectPoolSellable} from "../src/pool/ABProjectPoolSellable.sol";

contract TestableABProjectPoolSellable is ABProjectPoolSellable {
    constructor(
        Init memory init,
        GenArt721CoreV3_Engine_Flex_PROOF flex_,
        IGenArt721CoreContractV3_Mintable flexMintGateway_
    ) ABProjectPoolSellable(init, flex_, flexMintGateway_) {}

    uint128 public numProjects;

    mapping(uint128 => uint64) public maxNumPerProject;

    mapping(uint128 => bool) public isLongformProject;

    mapping(uint128 => uint256) public artblocksProjectId;

    function setNumProjects(uint128 numProjects_) public {
        numProjects = numProjects_;
    }

    function setMaxNumPerProject(uint128 projectId, uint64 maxNumPerProject_) public {
        maxNumPerProject[projectId] = maxNumPerProject_;
    }

    function setLongformProject(uint128 projectId, bool isLongformProject_) public {
        isLongformProject[projectId] = isLongformProject_;
    }

    function setArtblocksProjectId(uint128 projectId, uint256 artblocksProjectId_) public {
        artblocksProjectId[projectId] = artblocksProjectId_;
    }

    function _numProjects() internal view virtual override returns (uint128) {
        return numProjects;
    }

    function _maxNumPerProject(uint128 projectId) internal view virtual override returns (uint64) {
        return maxNumPerProject[projectId];
    }

    function _isLongformProject(uint128 projectId) internal view virtual override returns (bool) {
        return isLongformProject[projectId];
    }

    function _artblocksProjectId(uint128 projectId) internal view virtual override returns (uint256) {
        return artblocksProjectId[projectId];
    }

    function burn(uint256 tokenId) public {
        _burn(tokenId, true);
    }

    function nextTokenId() public view returns (uint256) {
        return _nextTokenId();
    }
}

contract ABPoolSetup is ArtblocksTest {
    address public immutable artist = makeAddr("artist");

    string public defaultURI = "uri://proof/";

    function setupABPool() public virtual returns (TestableABProjectPoolSellable) {
        TestableABProjectPoolSellable pool = new TestableABProjectPoolSellable(ProjectPoolSellable.Init({
                admin: admin,
                steerer: steerer,
                name: "name",
                symbol: "symbol",
                royaltyReciever: payable(address(0xFEE)),
                royaltyBasisPoints: 100,
                baseURI:defaultURI
            }), flex, IGenArt721CoreContractV3_Mintable(address(flex))  
        );

        vm.startPrank(steerer);
        // Adding some artblocks projects
        for (uint256 i; i < 100; ++i) {
            addActiveUnpausedProject(artist);
        }
        // allow pool to mint
        flex.updateMinterContract(address(pool));
        vm.stopPrank();

        return pool;
    }
}

contract ABProjectPoolSellableTest is ABPoolSetup {
    address public immutable seller = makeAddr("seller");

    TestableABProjectPoolSellable public pool;

    function setUp() public virtual override {
        super.setUp();

        pool = setupABPool();

        vm.startPrank(steerer);
        pool.grantRole(pool.AUTHORISED_SELLER_ROLE(), seller);
        vm.stopPrank();
    }

    function defaultTokenURI(uint256 tokenId) public view returns (string memory) {
        return string.concat(defaultURI, vm.toString(tokenId));
    }
}

contract ABPurchaseTest is ABProjectPoolSellableTest {
    uint128 constant LONG_PROJ_0 = 0;
    uint128 constant LONG_PROJ_1 = 3;
    uint128 constant NORM_PROJ_0 = 1;
    uint128 constant NORM_PROJ_1 = 2;
    uint128 constant LONG_AB_0 = 13;
    uint128 constant LONG_AB_1 = 37;

    function setUp() public virtual override {
        super.setUp();

        pool.setNumProjects(5);
        for (uint64 i; i < pool.numProjects(); ++i) {
            pool.setMaxNumPerProject(i, 2);
        }

        pool.setLongformProject(LONG_PROJ_0, true);
        pool.setLongformProject(LONG_PROJ_1, true);
        pool.setArtblocksProjectId(LONG_PROJ_0, LONG_AB_0);
        pool.setArtblocksProjectId(LONG_PROJ_1, LONG_AB_1);
    }

    function _purchase(address to, uint128[] memory projectIds, bool[] memory isLongformPurchase)
        internal
        assertERC721BalanceChanged(
            ERC721BalanceDelta({token: address(pool), account: to, delta: int256(projectIds.length)})
        )
        assertERC721TotalSupplyChangedBy(address(pool), int256(projectIds.length))
    {
        uint256 startTokenId = pool.totalSupply();

        vm.prank(seller);
        pool.handleSale(to, uint64(projectIds.length), PurchaseByProjectIDLib.encodePurchaseData(projectIds));

        for (uint256 i; i < projectIds.length; ++i) {
            uint256 tokenId = startTokenId + i;
            ABProjectPoolSellable.TokenInfo memory info = pool.tokenInfo(tokenId);
            assertEq(info.projectId, projectIds[i]);

            if (isLongformPurchase[i]) {
                assertEq(
                    flex.ownerOf(artblocksTokenID(pool.artblocksProjectId(info.projectId), info.edition)), address(pool)
                );
            }
        }
    }

    function assertDefaultURI(uint256 tokenId) public {
        assertEq(pool.tokenURI(tokenId), defaultTokenURI(tokenId));
    }

    function assertABURI(uint256 tokenId, uint256 projectId, uint256 edition) public {
        assertEq(pool.tokenURI(tokenId), artblocksTokenURI(projectId, edition));
    }

    function testSuccess() public {
        address to = makeAddr("to");
        _purchase(
            to, toUint128s([NORM_PROJ_0, LONG_PROJ_0, LONG_PROJ_1, NORM_PROJ_1]), toBools([false, true, true, false])
        );

        assertDefaultURI(0);
        assertABURI(1, LONG_AB_0, 0);
        assertABURI(2, LONG_AB_1, 0);
        assertDefaultURI(3);

        _purchase(to, toUint128s([LONG_PROJ_0, NORM_PROJ_0, NORM_PROJ_1]), toBools([true, false, false]));

        assertABURI(4, LONG_AB_0, 1);
        assertDefaultURI(5);
        assertDefaultURI(6);
    }
}

import {
    TestableProjectPoolSellable,
    PurchaseTest as PoolPurchaseTest_,
    BurnTest as PoolBurnTest_
} from "./ProjectPoolSellable.t.sol";

contract PoolPurchaseTest is PoolPurchaseTest_, ABPoolSetup {
    function setUp() public virtual override(PoolPurchaseTest_, ArtblocksTest) {
        ArtblocksTest.setUp();
        PoolPurchaseTest_.setUp();
    }

    function setupPool() public virtual override returns (TestableProjectPoolSellable) {
        return TestableProjectPoolSellable(address(ABPoolSetup.setupABPool()));
    }
}

contract PoolBurnTest is PoolBurnTest_, ABPoolSetup {
    function setUp() public virtual override(PoolBurnTest_, ArtblocksTest) {
        ArtblocksTest.setUp();
        PoolBurnTest_.setUp();
    }

    function setupPool() public virtual override returns (TestableProjectPoolSellable) {
        return TestableProjectPoolSellable(address(ABPoolSetup.setupABPool()));
    }
}
