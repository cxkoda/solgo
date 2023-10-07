// SPDX-License-Identifier: MIT
// Copyright (c) 2022 the ethier authors (github.com/divergencetech/ethier)
pragma solidity ^0.8.15;

import {Test} from "forge-std/Test.sol";

import {Address} from "openzeppelin-contracts/utils/Address.sol";
import {Strings} from "openzeppelin-contracts/utils/Strings.sol";
import {IERC20, ERC20} from "openzeppelin-contracts/token/ERC20/ERC20.sol";
import {IERC721, ERC721} from "openzeppelin-contracts/token/ERC721/ERC721.sol";
import {AccessControlEnumerable} from "ethier/utils/AccessControlEnumerable.sol";

import {Math} from "./Math.sol";

interface WithTotalSupply {
    function totalSupply() external view returns (uint256);
}

contract ProofTest is Test {
    using Address for address;

    bytes32 public immutable DEFAULT_ADMIN_ROLE;
    bytes32 public immutable DEFAULT_STEERING_ROLE;

    constructor() {
        AccessControlEnumerable ace = new AccessControlEnumerable();
        DEFAULT_ADMIN_ROLE = ace.DEFAULT_ADMIN_ROLE();
        DEFAULT_STEERING_ROLE = ace.DEFAULT_STEERING_ROLE();
    }

    address public immutable admin = makeAddr("admin");
    address public immutable steerer = makeAddr("steerer");

    function missingRoleError(address account, bytes32 role) public pure returns (bytes memory) {
        return bytes(
            string.concat(
                "AccessControl: account ", Strings.toHexString(account), " is missing role ", vm.toString(role)
            )
        );
    }

    function expectRevertNotSteererThenPrank(address vandal) public {
        vm.assume(vandal != steerer);
        vm.expectRevert(missingRoleError(vandal, DEFAULT_STEERING_ROLE));
        vm.prank(vandal, steerer);
    }

    function _assumeNotContract(address account) public view {
        vm.assume(uint160(account) > 10);
        vm.assume(!account.isContract());
    }

    modifier assertBalanceChangedBy(address account, int256 delta) {
        uint256 balance = account.balance;
        _;
        assertEq(account.balance, uint256(int256(balance) + delta));
    }

    modifier assertERC20BalanceChangedBy(ERC20BalanceChangedBy memory d) {
        uint256 balance = IERC20(d.token).balanceOf(d.account);
        _;
        assertEq(IERC20(d.token).balanceOf(d.account), uint256(int256(balance) + d.delta));
    }

    struct ERC20BalanceChangedBy {
        address token;
        address account;
        int256 delta;
    }

    modifier assertERC721BalanceChangedBy(IERC721 token, address account, int256 delta) {
        // Excluding this since balance checks of most ERC721 implementations will fail if the account is the zero
        // address.
        vm.assume(account != address(0));
        uint256 balance = token.balanceOf(account);
        _;
        assertEq(token.balanceOf(account), uint256(int256(balance) + delta));
    }

    struct ERC721BalanceDelta {
        address token;
        address account;
        int256 delta;
    }

    modifier assertERC721BalanceChanged(ERC721BalanceDelta memory d) {
        // Excluding this since balance checks of most ERC721 implementations will fail if the account is the zero
        // address.
        vm.assume(d.account != address(0));
        uint256 balance = IERC721(d.token).balanceOf(d.account);
        _;
        assertEq(IERC721(d.token).balanceOf(d.account), uint256(int256(balance) + d.delta));
    }

    modifier assertERC721TotalSupplyChangedBy(address token_, int256 delta) {
        uint256 supply = WithTotalSupply(token_).totalSupply();
        _;
        assertEq(WithTotalSupply(token_).totalSupply(), uint256(int256(supply) + delta));
    }

    modifier assertERC721TokenBurned(IERC721 token, uint256 tokenId) {
        // Checking that it did not revert before.
        token.ownerOf(tokenId);
        _;
        assertERC721Existence(address(token), tokenId, false);
    }

    modifier assertERC721TokenBurnedIf(address token, uint256 tokenId, bool condition) {
        if (!condition) {
            _;
            return;
        }

        // Checking that it did not revert before.
        IERC721(token).ownerOf(tokenId);
        _;
        assertERC721Existence(token, tokenId, false);
    }

    function assertERC721Existence(address token, uint256 tokenId, bool exists) public {
        // Not checking a specific revert here since the ERC721 standard does not mandate a specific error and it will
        // differ across implementations.
        // TODO (dave): explore EIP165 checks to determine the implementation
        if (!exists) {
            vm.expectRevert();
        }
        IERC721(token).ownerOf(tokenId);
    }

    function zeroIfErrElse(bytes memory err, uint256 x) public pure returns (uint256) {
        if (err.length == 0) {
            return x;
        }
        return 0;
    }

    function zeroIfErrElse(bytes memory err, int256 x) public pure returns (int256) {
        if (err.length == 0) {
            return x;
        }
        return 0;
    }

    function sequence(uint256 from, uint256 to) public pure returns (uint256[] memory) {
        uint256 num = to - from;
        uint256[] memory seq = new uint[](num);
        for (uint256 i = 0; i < num; i++) {
            seq[i] = from + i;
        }
        return seq;
    }

    function fillUint128s(uint256 num, uint128 value) public pure returns (uint128[] memory) {
        uint128[] memory arr = new uint128[](num);
        for (uint256 i = 0; i < num; i++) {
            arr[i] = value;
        }
        return arr;
    }

    function slice(uint256[] memory x, uint256 from, uint256 to) public pure returns (uint256[] memory) {
        uint256 num = to - from;
        uint256[] memory s = new uint256[](num);
        for (uint256 i = 0; i < num; i++) {
            s[i] = x[from + i];
        }
        return s;
    }

    function slice(uint8[] memory x, uint256 from, uint256 to) public pure returns (uint8[] memory) {
        uint256 num = to - from;
        uint8[] memory s = new uint8[](num);
        for (uint256 i = 0; i < num; i++) {
            s[i] = x[from + i];
        }
        return s;
    }

    /**
     * @notice Reduces the length of an array.
     */
    function shrink(uint256[] memory x, uint256 newLength) public pure {
        vm.assume(newLength <= x.length);
        assembly {
            mstore(x, newLength)
        }
    }

    /**
     * @notice Returns an array containing the elements of the input array marked by the input mask.
     */
    function select(uint256[] memory x, bool[] memory mask) public pure returns (uint256[] memory) {
        require(x.length == mask.length, "select: input length mismatch");
        uint256 num = mask.length;
        uint256[] memory ret = new uint[](num);

        uint256 cursor;
        for (uint256 i; i < num; ++i) {
            if (mask[i]) {
                ret[cursor++] = x[i];
            }
        }
        shrink(ret, cursor);

        return ret;
    }

    function copy(bool[] memory a) public pure returns (bool[] memory) {
        uint256 num = a.length;
        bool[] memory c = new bool[](num);
        for (uint256 i; i < num; ++i) {
            c[i] = a[i];
        }
        return c;
    }

    function and(bool[] memory a, bool[] memory b) public pure returns (bool[] memory) {
        assert(a.length == b.length);
        uint256 num = b.length;
        bool[] memory ret = new bool[](num);
        for (uint256 i; i < num; ++i) {
            ret[i] = a[i] && b[i];
        }
        return ret;
    }

    function not(bool[] memory a) public pure returns (bool[] memory) {
        uint256 num = a.length;
        bool[] memory ret = new bool[](num);
        for (uint256 i; i < num; ++i) {
            ret[i] = !a[i];
        }
        return ret;
    }

    function _deltasToAbsolute(uint256[] memory deltas, uint256 offset) private pure returns (uint256[] memory) {
        uint256[] memory tokenIds = new uint[](deltas.length);
        uint256 tokenId = 0;
        for (uint256 i = 0; i < deltas.length; i++) {
            tokenId += uint256(deltas[i]) + offset;
            tokenIds[i] = tokenId;
        }
        return tokenIds;
    }

    function deltasToAbsolute(uint256[] memory deltas) public pure returns (uint256[] memory) {
        return _deltasToAbsolute(deltas, 0);
    }

    function deltasToUniqueAbsolute(uint256[] memory deltas) public pure returns (uint256[] memory) {
        return _deltasToAbsolute(deltas, 1);
    }

    function assertBoundedIncl(uint256 x, uint256 lower, uint256 upper, string memory reason) public {
        string memory err =
            string.concat(reason, " ", vm.toString(x), " not in [", vm.toString(lower), ",", vm.toString(upper), "]");

        assertLe(x, upper, err);
        assertGe(x, lower, err);
    }

    function assertBoundedIncl(uint256 x, uint256 lower, uint256 upper) public {
        assertBoundedIncl(x, lower, upper, "");
    }

    function assertBinomialBoundedIncl(uint256 x, uint256 mean, uint256 numSigma) public {
        assertBinomialBoundedIncl(x, mean, numSigma, "");
    }

    function assertBinomialBoundedIncl(uint256 x, uint256 mean, uint256 numSigma, string memory reason) public {
        // crude approximation of the confidence interval via the central limit theorem
        // the approximation is only good if mean is sufficiently large
        // https://en.wikipedia.org/wiki/Binomial_proportion_confidence_interval#Normal_approximation_interval
        uint256 tolerance = numSigma * Math.intSqrt(mean);
        assertBoundedIncl(x, mean > tolerance ? mean - tolerance : 0, mean + tolerance, reason);
    }

    function quickShuffle(uint256[] memory arr, uint256 rand) public pure returns (uint256[] memory) {
        if (arr.length < 2) {
            return arr;
        }

        uint256 prime = 11_807;
        vm.assume(arr.length < prime);
        rand = rand % arr.length;

        uint256[] memory shuffled = new uint[](arr.length);
        for (uint8 i = 0; i < arr.length; i++) {
            shuffled[i] = arr[(prime * (rand + i)) % arr.length];
        }
        return shuffled;
    }

    // TODO(dave): generated these programmatically via a go template and add more.

    function assertEq(uint64[] memory got, uint64[] memory want) public {
        assertEq(toUint256s(got), toUint256s(want));
    }

    function toBools(bool[1] memory input) public pure returns (bool[] memory output) {
        output = new bool[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = bool(input[i]);
        }
    }

    function toBools(bool[2] memory input) public pure returns (bool[] memory output) {
        output = new bool[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = bool(input[i]);
        }
    }

    function toBools(bool[3] memory input) public pure returns (bool[] memory output) {
        output = new bool[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = bool(input[i]);
        }
    }

    function toBools(bool[4] memory input) public pure returns (bool[] memory output) {
        output = new bool[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = bool(input[i]);
        }
    }

    function toUint8s(uint256[] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = uint8(input[i]);
        }
    }

    function toUint8s(uint8[1] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint8s(uint8[2] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint8s(uint8[3] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint8s(uint8[4] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint8s(uint8[5] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint8s(uint8[10] memory input) public pure returns (uint8[] memory output) {
        output = new uint8[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint8[1] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint8[2] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint8[3] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint8[4] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint8[5] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint32[1] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint32[2] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint32[3] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint32[4] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint64[1] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint64[2] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint64[3] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint64s(uint64[4] memory input) public pure returns (uint64[] memory output) {
        output = new uint64[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[1] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[2] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[3] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[4] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[5] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[6] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint8[7] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint128[1] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint128[2] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint128[3] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint128[4] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint128s(uint128[5] memory input) public pure returns (uint128[] memory output) {
        output = new uint128[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[1] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[2] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[3] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[4] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[5] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[6] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[7] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint8[20] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[1] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[2] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[3] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[4] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[5] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint16[6] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint32[20] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint32[50] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint32[100] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint64[] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint128[1] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint128[2] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint128[3] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint128[4] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint256[1] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toUint256s(uint256[4] memory input) public pure returns (uint256[] memory output) {
        output = new uint256[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }

    function toAddresses(address[1] memory input) public pure returns (address[] memory output) {
        output = new address[](input.length);
        for (uint256 i; i < input.length; ++i) {
            output[i] = input[i];
        }
    }
}

contract SignerTest is Test {
    address public immutable signer;
    uint256 public immutable signerKey;

    constructor() {
        (signer, signerKey) = makeAddrAndKey("signer");
    }

    function _sign(uint256 key, bytes32 digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(key, digest);
        return abi.encodePacked(r, s, v);
    }
}

contract ERC721Fake is ERC721("Fake", "FAKE") {
    uint256[] private _tokenIds;

    uint256 public numMinted;

    uint256 public numBurned;

    function tokenIds() public view returns (uint256[] memory) {
        return _tokenIds;
    }

    function totalSupply() public view returns (uint256) {
        return numMinted - numBurned;
    }

    function mint(address to, uint256 tokenId) public {
        _mint(to, tokenId);
        _tokenIds.push(tokenId);
        numMinted++;
    }

    function mint(address to, uint256[] memory tokenIds_) public {
        for (uint256 i; i < tokenIds_.length; ++i) {
            mint(to, tokenIds_[i]);
        }
    }

    function burn(uint256 tokenId) public virtual {
        _burn(tokenId);
        numBurned++;
    }
}

contract ERC20Fake is ERC20 {
    constructor() ERC20("Fake", "FAKE") {}

    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }
}