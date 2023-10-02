"""Macros for use with @aspect_rules_sol."""

load("@aspect_rules_sol//sol:repositories.bzl", "LATEST_VERSION", "sol_register_toolchains")
load("@aspect_rules_sol//sol:providers.bzl", "SolRemappingsInfo")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "new_git_repository")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@io_bazel_rules_go//go:def.bzl", "GoArchive", "GoLibrary", "GoSource", "go_context")

def sol_register_0_8_toolchains():
    """Register `solc_latest` and all `solc_0-8-*` toolchains."""
    sol_register_toolchains(
        name = "solc_latest",
        sol_version = LATEST_VERSION,
    )

    LATEST_PATCH = int(LATEST_VERSION.split(".")[-1])
    for patch in range(0, LATEST_PATCH + 1):
        sol_register_toolchains(
            name = "solc_0-8-%d" % patch,
            sol_version = "0.8.%d" % patch,
        )

def sol_http_archive(name, remappings = {}, **kwargs):
    """Load an http_archive() of Solidity files, exposing a sol_sources() target of all .sol files.

    Args:
      name: Name of the sol_sources() target; reference as @name when used in
        deps of a sol_binary(); e.g. @openzeppelin-contracts_4-8-1.
      remappings: solc import remappings with the value relative to the root of
        the repository; e.g. {"@openzeppelin/contracts/": "/contracts"} would
        result in all imports beginning with @openzeppelin/contracts being
        sourced from the contracts directory of this repository. Leading slash
        is optional.
      **kwargs: Propagated to http_archive().
    """
    http_archive(
        name = name,
        build_file_content = _build_file_content(name, remappings),
        **kwargs
    )

def sol_git_repository(name, remappings = {}, **kwargs):
    """Load a git repository of Solidity files, exposing a sol_sources() target of all .sol files.

    Where possible, prefer sol_http_repository() as there are limitations to
    caching of new_git_repository().

    Args:
      name: Name of the sol_sources() target; reference as @name when used in
        deps of a sol_binary(); e.g. @openzeppelin-contracts_4-8-1.
      remappings: solc import remappings with the value relative to the root of
        the repository; e.g. {"@openzeppelin/contracts/": "/contracts"} would
        result in all imports beginning with @openzeppelin/contracts being
        sourced from the contracts directory of this repository. Leading slash
        is optional.
      **kwargs: Propagated to new_git_repository().
    """
    new_git_repository(
        name = name,
        build_file_content = _build_file_content(name, remappings),
        **kwargs
    )

def _build_file_content(name, remappings):
    remappings = {
        k: "/".join([".", "external", name, v.removeprefix("/")])  # ./external/{name}/{v}
        for (k, v) in remappings.items()
    }

    return """\
load("@aspect_rules_sol//sol:defs.bzl", "sol_sources")

package(default_visibility = ["//visibility:public"])

sol_sources(
    name = "{name}",
    srcs = glob(["**/*.sol"]),
    remappings = {remappings}
)
""".format(name = name, remappings = remappings)

SOL_GO_LIBRARY_DEPS = [
    "@com_github_ethereum_go_ethereum//:go-ethereum",
    "@com_github_ethereum_go_ethereum//accounts/abi",
    "@com_github_ethereum_go_ethereum//accounts/abi/bind",
    "@com_github_ethereum_go_ethereum//common",
    "@com_github_ethereum_go_ethereum//core/types",
    "@com_github_ethereum_go_ethereum//event",
]

def _sol_go_library_impl(ctx):
    # TODO: modify sol_binary to provide a new SolBinaryInfo provider that
    # explicitly carries a combined.json. For now we just find it at "runtime".
    combined_json = ""
    for f in ctx.attr.binary.files.to_list():
        if f.basename == "combined.json":
            if combined_json != "":
                fail("multiple combined.json files in sol_go_library.binary")
            combined_json = f

    args = ctx.actions.args()
    args.add("--lang", "go")
    args.add("--pkg", ctx.attr.pkg)
    args.add("--combined-json", combined_json.path)

    abigen_out = ctx.actions.declare_file("%s.sol.go" % ctx.attr.name)
    args.add("--out", abigen_out)

    ctx.actions.run(
        inputs = depset([combined_json]),
        outputs = [abigen_out],
        arguments = [args],
        executable = ctx.executable._abigen,
    )

    go = go_context(ctx)
    golib = go.new_library(
        go,
        srcs = [abigen_out],
        importable = True,
    )
    gosrc = go.library_to_source(
        go,
        attr = ctx.attr,
        library = golib,
        coverage_instrumented = ctx.coverage_instrumented(),
    )
    return [golib, gosrc, go.archive(go, source = gosrc)]

sol_go_library = rule(
    doc = """Generate Solidity Go bindings using abigen. This target is embeddable in a go_library / go_binary.

Note that Gazelle is unaware of sol_go_library(). The target must therefore be
embedded with a #keep to avoid it being removed. If the embed is the only embed
and no src is provided, then the embedding target's importpath must also be
tagged with #keep.

Example usage:
```
    sol_binary(
        name = "nft_sol",
        srcs = [
            "MyNFT.sol",
            "MyFancyStakingMechanism.sol",
        ],
        pkg = "nft",
        deps = ["@openzeppelin-contracts_4-8-1"], # see sol_git_repository
    )

    sol_go_library(
        name = "nft_sol_go",
        binary = ":nft_sol",
        pkg = "nft",
    )

    go_library(
        name = "nft",
        embed = [
            ":nft_sol_go", #keep
        ],
        importpath = "github.com/org/repo/path/to/nft", #keep
    )

    go_test(
        name = "nft_test",
        embed = [
            ":nft_sol", #keep
            # and/or embed [":nft"] with #keep as necessary
        ],
    )
```
""",
    implementation = _sol_go_library_impl,
    attrs = {
        "binary": attr.label(
            doc = "The sol_binary target producing a combined.json for which `abigen` bindings are to be generated.",
            providers = [[DefaultInfo, SolRemappingsInfo]],
            mandatory = True,
        ),
        "pkg": attr.string(
            doc = "Propagated to abigen --pkg",
            mandatory = True,
        ),
        "deps": attr.label_list(
            default = SOL_GO_LIBRARY_DEPS,
        ),
        "_abigen": attr.label(
            default = Label("@com_github_ethereum_go_ethereum//cmd/abigen"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "_go_context_data": attr.label(
            # https://github.com/bazelbuild/rules_go/blob/master/go/toolchains.rst#writing-new-go-rules
            default = "@io_bazel_rules_go//:go_context_data",
        ),
    },
    provides = [GoArchive, GoLibrary, GoSource],
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)
