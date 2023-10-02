"""Macro to bundle JavaScript and create a binary."""

load("@aspect_rules_esbuild//esbuild:defs.bzl", "esbuild")
load("@aspect_rules_js//js:defs.bzl", "js_binary")

def js_bundled_binary(name, srcs, entry_point, visibility = ["//visibility:private"], **kwargs):
    """Bundles srcs with esbuild, using the output as a js_binary entry point.

    Args:
        name: Name of the js_binary target to be created.
        srcs: Sources compatible with the esbuild rule.
        entry_point: Propagated to the esbuild target.
        visibility: Propagated to the js_binary target.
        **kwargs: Propagated to the esbuild target. For convenience, the
            platform field defaults to "node".
    """
    if "platform" not in kwargs or kwargs["platform"] == "":
        kwargs["platform"] = "node"

    CONFIG_TARGET = "_%s_esbuild_config" % name
    CONFIG = "%s_esbuild.config.mjs" % name
    native.genrule(
        name = CONFIG_TARGET,
        srcs = ["//bazel/js_bundled_binary:esbuild.config.mjs"],
        outs = [CONFIG],
        cmd_bash = """
        cp $(location //bazel/js_bundled_binary:esbuild.config.mjs) $@
        """,
    )

    BUNDLE_TARGET = "_%s_esbuild" % name
    BUNDLE = "%s_bundle.js" % name
    esbuild(
        name = BUNDLE_TARGET,
        srcs = srcs,
        entry_point = entry_point,
        output = BUNDLE,
        config = CONFIG,
        **kwargs
    )

    js_binary(
        name = name,
        entry_point = BUNDLE,
        visibility = visibility,
    )
