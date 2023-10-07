// SPDX-License-Identifier: MIT
// Copyright 2023 Proof Holdings Inc.
pragma solidity >=0.8.17;

import {ProofTest} from "proof/constants/Testing.sol";

import {ERC721A, ERC721ACommon} from "ethier/erc721/BaseTokenURI.sol";

import {
    PurchaseByProjectIDLib,
    TokenInfoManager,
    SellableERC721ACommonByProjectID
} from "../src/sellable/SellableERC721ACommonByProjectID.sol";

contract PurchaseDataTest is ProofTest {
    function testRoundTrip(uint128[] memory want) public {
        uint128[] memory got =
            PurchaseByProjectIDLib.decodePurchaseData(PurchaseByProjectIDLib.encodePurchaseData(want));
        assertEq(keccak256(abi.encode(got)), keccak256(abi.encode(want)));
    }
}

contract Helper is ProofTest {
    function assertEq(TokenInfoManager.TokenInfo memory got, TokenInfoManager.TokenInfo memory want) public {
        assertEq(got.projectId, want.projectId, "wrong projectId");
        assertEq(got.edition, want.edition, "wrong edition");
        assertEq(got.extra, want.extra, "wrong extra");
    }
}

contract TestableTokenInfoManager is TokenInfoManager {
    function setTokenInfo(uint256 tokenId, TokenInfo memory info) public {
        _setTokenInfo(tokenId, info);
    }
}

contract TokenInfoManagerTest is ProofTest, Helper {
    TestableTokenInfoManager public manager;

    function setUp() public {
        manager = new TestableTokenInfoManager();
    }

    function testSetTokenInfo(TokenInfoManager.TokenInfo[5] memory want, uint16[5] memory tokenIdsDeltas) public {
        uint256[] memory tokenIds = deltasToUniqueAbsolute(toUint256s(tokenIdsDeltas));

        for (uint256 i; i < want.length; ++i) {
            manager.setTokenInfo(tokenIds[i], want[i]);
        }

        TokenInfoManager.TokenInfo[] memory got = manager.tokenInfos(tokenIds);
        for (uint256 i; i < want.length; ++i) {
            assertEq(got[i], want[i]);
        }
    }
}

contract TestableSellableERC721ACommonByProjectID is SellableERC721ACommonByProjectID {
    constructor(address admin, address steerer) ERC721ACommon(admin, steerer, "", "", payable(address(0xFEE)), 100) {}

    function nextTokenId() public view returns (uint256) {
        return _nextTokenId();
    }

    mapping(bytes32 => bool) internal _projectMintHandled;

    function projectMintHandled(uint256 tokenId, uint128 projectId, uint64 edition) public view returns (bool) {
        return _projectMintHandled[keccak256(abi.encode(tokenId, projectId, edition))];
    }

    function _handleProjectMinted(uint256 tokenId, uint128 projectId, uint64 edition) internal virtual override {
        _projectMintHandled[keccak256(abi.encode(tokenId, projectId, edition))] = true;
    }
}

contract SellableERC721ACommonByProjectIDTest is ProofTest, Helper {
    TestableSellableERC721ACommonByProjectID public sellable;

    address public immutable seller = makeAddr("seller");

    function setUp() public {
        sellable = new TestableSellableERC721ACommonByProjectID(admin, steerer);

        vm.startPrank(steerer);
        sellable.grantRole(sellable.AUTHORISED_SELLER_ROLE(), seller);
        vm.stopPrank();
    }

    mapping(uint256 => uint64) public nextEdition;

    function _handleSale(address to, uint128[] memory projectIds)
        internal
        assertERC721BalanceChanged(
            ERC721BalanceDelta({token: address(sellable), account: to, delta: int256(projectIds.length)})
        )
        assertERC721TotalSupplyChangedBy(address(sellable), int256(projectIds.length))
    {
        uint256 num = projectIds.length;
        uint256 tokenId = sellable.nextTokenId();

        vm.prank(seller);
        sellable.handleSale(to, uint64(num), PurchaseByProjectIDLib.encodePurchaseData(projectIds));

        for (uint256 i; i < num; ++i) {
            {
                TokenInfoManager.TokenInfo memory info = sellable.tokenInfo(tokenId + i);
                assertEq(info.projectId, projectIds[i]);
                assertEq(info.edition, nextEdition[projectIds[i]]);
            }

            assertTrue(sellable.projectMintHandled(tokenId + i, projectIds[i], nextEdition[projectIds[i]]));

            nextEdition[projectIds[i]]++;
        }

        for (uint256 i; i < num; ++i) {
            // checking this in a separate loop so that `nextEdition` is already updated for all projects and
            // corresponds to the number of tokens minted for each project
            assertEq(sellable.numPurchasedPerProject(projectIds[i]), nextEdition[projectIds[i]]);
        }
    }

    function testSale() public {
        address alice = makeAddr("alice");
        _handleSale(alice, toUint128s([3, 5, 3, 156]));

        TokenInfoManager.TokenInfo[] memory info = sellable.tokenInfos(sequence(0, 4));
        assertEq(info[0], TokenInfoManager.TokenInfo({projectId: 3, edition: 0, extra: bytes8(0)}));
        assertEq(info[1], TokenInfoManager.TokenInfo({projectId: 5, edition: 0, extra: bytes8(0)}));
        assertEq(info[2], TokenInfoManager.TokenInfo({projectId: 3, edition: 1, extra: bytes8(0)}));
        assertEq(info[3], TokenInfoManager.TokenInfo({projectId: 156, edition: 0, extra: bytes8(0)}));
    }

    function testSale(address to, uint128[] memory projectIds) public {
        _assumeNotContract(to);
        vm.assume(projectIds.length > 0);
        _handleSale(to, projectIds);
    }

    function testSaleRepeated(address[2] memory to, uint128[][2] memory projectIds) public {
        _assumeNotContract(to[0]);
        _assumeNotContract(to[1]);
        vm.assume(projectIds[0].length > 0);
        vm.assume(projectIds[1].length > 0);

        _handleSale(to[0], projectIds[0]);
        _handleSale(to[1], projectIds[1]);
    }

    function testWrongProjectsLength(address to, uint64 num, uint128[] memory projectIds) public {
        vm.assume(num != projectIds.length);
        vm.expectRevert();
        vm.prank(seller);
        sellable.handleSale(to, num, PurchaseByProjectIDLib.encodePurchaseData(projectIds));
    }
}
