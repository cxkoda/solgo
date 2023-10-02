"""Foundry Rust dependencies

This enables Forge to be accessed via `bazel run @foundry//:foundry-cli__forge`
"""

load("@rules_rust//crate_universe:defs.bzl", "crate", "crates_repository")
load("@aspect_rules_sol//sol:providers.bzl", "SolSourcesInfo")

def sol_foundry_repositories():
    crates_repository(
        name = "foundry",
        annotations = {
            "foundry-cli": [crate.annotation(
                gen_binaries = True,
                shallow_since = "1688226435 -0400",
            )],
            "svm-rs": [crate.annotation(
                # When there are two binaries defined in a Cargo.toml, the Bazel rule
                # is suffixed with __bin while other Bazel rules still depend on the name
                # without the suffix. Create this alias for other BUILD files. Relevant Cargo.toml:
                # https://github.com/alloy-rs/svm-rs/blob/master/crates/svm-rs/Cargo.toml#L17-L25
                additive_build_file_content = """
                    alias(
                        name = "svm",
                        actual = "@foundry__svm-rs-0.2.23//:svm__bin",
                    )
                """,
                # This binary is a dependency for Forge.
                gen_binaries = True,
                # These must be listed to force load optional dependencies:
                # https://github.com/alloy-rs/svm-rs/blob/master/crates/svm-rs/Cargo.toml#L41-L47
                deps = [
                    "@foundry//:anyhow",
                    "@foundry//:clap",
                    "@foundry//:console",
                    "@foundry//:dialoguer",
                    "@foundry//:indicatif",
                    "@foundry//:itertools",
                    "@foundry//:tokio",
                ],
            )],
            "svm-rs-builds": [crate.annotation(
                build_script_deps = [
                    "@foundry__svm-rs-0.2.23//:svm_lib",
                ],
                patches = ["@proof//BUILD-patches:foundry_svm-rs-builds.patch"],
                deps = [
                    # Relative path dependency needs to be depended on explicitly
                    # https://github.com/alloy-rs/svm-rs/blob/master/crates/svm-builds/Cargo.toml#L17
                    "@foundry//:svm-rs",
                ],
            )],
            "ethers": [crate.annotation(shallow_since = "1685722078 -0700")],
            "ethers-core": [crate.annotation(
                patches = ["@proof//BUILD-patches:foundry_ethers-core.patch"],
            )],
            "ethers-contract-abigen": [crate.annotation(
                patches = ["@proof//BUILD-patches:foundry_ethers-contract-abigen.patch"],
            )],
            "ethers-etherscan": [crate.annotation(
                deps = [
                    # Relative path dependency needs to be depended on explicitly
                    # https://github.com/gakonst/ethers-rs/blob/master/ethers-etherscan/Cargo.toml#L31
                    "@foundry//:ethers-solc",
                ],
            )],
            "ethers-solc": [crate.annotation(
                patches = ["@proof//BUILD-patches:foundry_ethers-solc.patch"],
                deps = [
                    "@foundry__svm-rs-0.2.23//:svm_lib",
                ],
            )],
            "foundry-abi": [crate.annotation(
                deps = [
                    "@foundry__ethers-2.0.7//:ethers",
                ],
            )],
        },
        cargo_lockfile = "//:Cargo.lock",
        lockfile = "//:Cargo.Bazel.lock",
        packages = {
            "foundry-cli": crate.spec(
                git = "https://github.com/foundry-rs/foundry",
                rev = "d369d2486f85576eec4ca41d277391dfdae21ba7",
            ),
            "forge": crate.spec(
                git = "https://github.com/foundry-rs/foundry",
                rev = "d369d2486f85576eec4ca41d277391dfdae21ba7",
            ),
            # Required for svm-rs-builds:
            "svm-rs": crate.spec(
                git = "https://github.com/alloy-rs/svm-rs",
                rev = "3f99e177de86d3756c799c0a23eea0fe6be55280",
                features = ["rustls", "cli"],
            ),
            # Pin svm-rs-builds' version:
            "svm-rs-builds": crate.spec(
                git = "https://github.com/alloy-rs/svm-rs",
                # svm-rs-builds@0.1.15
                rev = "3f99e177de86d3756c799c0a23eea0fe6be55280",
            ),
            # Force loading of optional dependencies for `svm-rs`:
            "anyhow": crate.spec(version = "1.0.72"),
            "clap": crate.spec(
                features = ["derive"],
                version = "4.3.21",
            ),
            "console": crate.spec(version = "0.15.7"),
            "dialoguer": crate.spec(version = "0.10.4"),
            "indicatif": crate.spec(),
            "itertools": crate.spec(
                features = ["use_std"],
                version = "0.10.5",
            ),
            "tokio": crate.spec(
                features = [
                    "rt-multi-thread",
                    "macros",
                ],
                version = "1.31.0",
            ),
            # Required for foundry_abi:
            "ethers": crate.spec(
                features = [
                    "ledger",
                    "trezor",
                    "ethers-solc",
                ],
                git = "https://github.com/gakonst/ethers-rs/",
                rev = "7b7c62327303866319a8b9ea84de517eef70dc09",
            ),
            # Must define "full" to pull in all required dependencies.
            "ethers-solc": crate.spec(
                features = ["full"],
                git = "https://github.com/gakonst/ethers-rs/",
                rev = "7b7c62327303866319a8b9ea84de517eef70dc09",
            ),
            # Must define "aws" to pull in all required dependencies.
            "ethers-signers": crate.spec(
                features = ["aws"],
                git = "https://github.com/gakonst/ethers-rs/",
                rev = "7b7c62327303866319a8b9ea84de517eef70dc09",
            ),
        },
    )

def _sol_forge_test_impl(ctx):
    project_dir = ctx.build_file_path.split("/BUILD.bazel")[0]

    content = """
if [ -f {project_dir}/runfile_remappings.txt ]; then
    mv -f {project_dir}/runfile_remappings.txt {project_dir}/remappings.txt
fi
{forge_binary} build --root {project_dir}
{forge_binary} test -vvv --root {project_dir}
""".format(forge_binary = ctx.executable._forge_binary.short_path, project_dir = project_dir)

    for dep in ctx.attr.deps:
        for file in dep.files.to_list():
            # One can pass in a name that doesn't exist and will have a single file with no extension.
            # Error if that's the case.
            if not file.extension:
                fail("Dependency %s contains %s that has no extension. Ensure the name of the dependency is correct and it contains files." % (dep.label, file.basename))

    script_file = ctx.actions.declare_file(ctx.label.name + ".sh")
    ctx.actions.write(
        output = script_file,
        content = content,
        is_executable = True,
    )

    # Fetch files in transitive dependencies that are sol sources
    transitive_sources = depset(
        transitive = [
            d[SolSourcesInfo].transitive_sources
            for d in ctx.attr.srcs
            if SolSourcesInfo in d
        ],
    )

    remappings = None
    for dep in ctx.files.deps:
        if dep.owner.name == "sol_forge_remappings":
            remappings = dep
            break

    return DefaultInfo(
        executable = script_file,
        runfiles = ctx.runfiles(
            files = ctx.files.srcs + [ctx.executable._forge_binary, remappings, ctx.file.foundrytoml],
            transitive_files = transitive_sources,
        ),
    )

_sol_forge_test = rule(
    implementation = _sol_forge_test_impl,
    attrs = {
        "srcs": attr.label_list(allow_files = True, providers = [[SolSourcesInfo]]),
        "deps": attr.label_list(allow_files = True),
        "foundrytoml": attr.label(allow_single_file = [".toml"]),
        "_forge_binary": attr.label(
            cfg = "exec",
            executable = True,
            default = "@foundry//:foundry-cli__forge",
        ),
    },
    test = True,
)

def sol_forge_test(name, srcs, deps, foundrytoml):
    _sol_forge_test(
        name = name,
        srcs = srcs,
        deps = deps,
        foundrytoml = foundrytoml,
        tags = ["local", "manual"],
    )
