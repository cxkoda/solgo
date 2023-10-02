"""Solidity dependencies"""

load("//bazel/sol:defs.bzl", "sol_git_repository", "sol_http_archive")

def sol_repositories():
    """Add all Solidity dependencies"""

    # Creates @ethier_{maj}-{min}-{patch}
    _ethier({
        "0.54.0": "4924d53a4ff34630c78109c1cf2ba9973557194086fa483a48777ae8c299791e",
        "0.55.0": "4bd40511d10c3fd6deb9bdb98a359fcbc5f8824427bb5d5127e9dd4748dede91",
    })

    # Creates @openzeppelin-contracts_{maj}-{min}-{patch}
    _openzeppelin({
        "4.7.0": "59b8e696611b950f827940d118d13e96583cbcc5eebe0cd69a1f288f0bb734b7",
        "4.7.3": "f782179ccecbcb7f8b40675722f8617293d6231b59322ae2e299106df2a38075",
        "4.8.1": "475798820b67d2947c3f68adbf22638add419ff479438673590998624be9baf9",
    })

    # Creates @openzeppelin-contracts-upgradeable_{maj}-{min}-{patch}
    _openzeppelin_upgradeable({
        "4.9.0": "bd25a009a12ee7c3dffa2c7b57ead94a7f3cbe14f8ea7df702603f71b9171378",
    })

    # Creates @ERC721A_{maj}-{min}-{patch}
    _erc721a({
        "4.2.3": "f5f030bd0e829ede9f3484f0eedf5ef4de5280f77d98d9f77cb80ed7bef50d72",
    })

    # Creates @delegation-registry_{commit[:8]}
    _delegationRegistry([
        "2d1a158b012f1d3ac138335c719d45fda0fa1d29",
    ])

    # Creates @artblocks_{maj}-{min}-{patch}
    _artblocks([
        "fa1dc46667c33f89163acad78e25ef781bc1c33a",
    ])

    # Creates @operator-filter-registry_{maj}-{min}-{patch}
    _operator_filter_registry({
        "1.4.1": "25287dd465161956e38397984129a3318f0ebf3f25c293ec81969db55fde988d",
    })

    # Creates @forge-std_{maj}-{min}-{patch}
    _forgeStd({
        "1.4.0": "706ebab17e703547184be55a5b445d77514df2a381c32acdb5c72c6a900fbb35",
        "1.5.6": "166faf7f7ad247efec89a65c109c742046228a6e9b8c6fb1f00d508f9b435c0a",
    })

    # Creates @ds-test_{commit[:8]}
    _dsTest([
        "013e6c6451effc6055c69384d056eff9b18b8646",
    ])

def _ethier(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")
        sol_http_archive(
            name = "ethier_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/divergencetech/ethier/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                # PROOF style
                "ethier/": "/contracts/",
                "ethier_root/": "/",
                # npm compat
                "@divergencetech/ethier": "/",
            },
            strip_prefix = "ethier-%s" % version,
        )

def _openzeppelin(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")

        sol_http_archive(
            name = "openzeppelin-contracts_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/OpenZeppelin/openzeppelin-contracts/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                # root import compat
                "openzeppelin-contracts/contracts/": "/contracts/",
                # PROOF style
                "openzeppelin-contracts/": "/contracts/",
                "openzeppelin-contracts_root/": "/",
                # npm compat
                "@openzeppelin/": "/",
            },
            strip_prefix = "openzeppelin-contracts-%s" % version,
        )

        remappings = {
            "openzeppelin-contracts-%s/" % _dashedVer(version): "/contracts/",
            "openzeppelin-contracts_root-%s/" % _dashedVer(version): "/",
            "@openzeppelin-%s/" % _dashedVer(version): "/",
        }
        if version.endswith(".0"):
            remappings["@openzeppelin-%s/" % version[:-2]] = "/"

        sol_http_archive(
            name = "openzeppelin-contracts_%s_exact_remap" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/OpenZeppelin/openzeppelin-contracts/archive/refs/tags/v%s.tar.gz" % version],
            remappings = remappings,
            strip_prefix = "openzeppelin-contracts-%s" % version,
        )

def _openzeppelin_upgradeable(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")

        sol_http_archive(
            name = "openzeppelin-contracts-upgradeable_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/OpenZeppelin/openzeppelin-contracts-upgradeable/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                # root import compat
                "openzeppelin-contracts-upgradeable/contracts/": "/contracts/",
                # PROOF style
                "openzeppelin-contracts-upgradeable/": "/contracts/",
                "openzeppelin-contracts-upgradeable_root/": "/",
                # npm compat
                "@openzeppelin/contracts-upgradeable": "/",
            },
            strip_prefix = "openzeppelin-contracts-upgradeable-%s" % version,
        )

def _erc721a(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")
        sol_http_archive(
            name = "ERC721A_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/chiru-labs/ERC721A/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                # PROOF style
                "ERC721A/": "/contracts/",
                "ERC721A_root/": "/",
                # npm compat
                "erc721a/": "/",
            },
            strip_prefix = "ERC721A-%s" % version,
        )

def _delegationRegistry(commits):
    for commit in commits:
        sol_git_repository(
            name = "delegation-registry_%s" % commit[:8],
            remote = "https://github.com/delegatecash/delegation-registry.git",
            commit = commit,
            remappings = {
                "delegation-registry/": "/src/",
                "delegation-registry_root/": "/",
            },
            shallow_since = "1677450996 -0500",
        )

def _artblocks(commits):
    for commit in commits:
        sol_git_repository(
            name = "artblocks-contracts_%s" % commit[:8],
            remote = "https://github.com/ArtBlocks/artblocks-contracts.git",
            commit = commit,
            remappings = {
                # PROOF style
                "artblocks-contracts/": "/contracts/",
                "artblocks-contracts_root/": "/",
            },
            shallow_since = "1680053181 -0700",
        )

def _operator_filter_registry(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")
        sol_http_archive(
            name = "operator-filter-registry_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/ProjectOpenSea/operator-filter-registry/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                # root import compat
                "operator-filter-registry/src/": "/src/",
                # PROOF style
                # Leaving this out for now to solve a remapping conflict where this is preferred over the root import
                # TODO(dave) explore other solutions to fix this.
                # "operator-filter-registry/": "/src/",
                "operator-filter-registry_root/": "/",
            },
            strip_prefix = "operator-filter-registry-%s" % version,
        )

def _forgeStd(versionHashes):
    for (version, sha256) in versionHashes.items():
        version = version.removeprefix("v")
        sol_http_archive(
            name = "forge-std_%s" % _dashedVer(version),
            sha256 = sha256,
            urls = ["https://github.com/foundry-rs/forge-std/archive/refs/tags/v%s.tar.gz" % version],
            remappings = {
                "forge-std/": "/src/",
                "forge-std_root/": "/",
            },
            strip_prefix = "forge-std-%s" % version,
        )

def _dsTest(commits):
    for commit in commits:
        sol_git_repository(
            name = "ds-test_%s" % commit[:8],
            remote = "https://github.com/dapphub/ds-test.git",
            commit = commit,
            remappings = {
                "ds-test/": "/src/",
                "ds-test_root/": "/",
            },
            shallow_since = "1676879876 +0100",
        )

def _dashedVer(v):
    """Replace all . with - in a semantic version."""
    return v.replace(".", "-")
