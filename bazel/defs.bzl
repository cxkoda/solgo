"""Utility functions and default-valued macros.
"""

load("@aspect_rules_ts//ts:defs.bzl", _ts_project = "ts_project")

def workspace_rel_path():
    "Returns the relative path from the current package to the workspace root."
    depth = len(native.package_name().split("/"))
    return "/".join([".."] * depth)

def ts_project(**kwargs):
    """Creates a @rules_ts ts_project resolving @proof/* to the workspace root.

    This allows workspace-relative imports instead of module-relative ones that
    require a multitude of ../ directory jumping. This aids in refactoring,
    makes imports more readable, and enables simpler IDE integration for module
    resolution.

    Args:
        **kwargs: Passed through to a regular @rules_ts ts_project after setting
        the compilerOptions.{baseUrl,paths} values. The tsconfig attribute MUST
        be present and MUST be a dict. There is no restriction on the tsconfig
        value except that compilerOptions.{baseUrl,paths} MUST NOT be set.
    """
    CONFIG = "tsconfig"
    if CONFIG not in kwargs or type(kwargs[CONFIG]) != "dict":
        # Having tsconfig as a dict in the BUILD file ensures that it is the
        # source of truth, instead of relying on the tsconfig.json file, which
        # we keep for IDE integration only.
        fail("ts_project.%s must be a dict" % CONFIG)

    OPTS = "compilerOptions"
    BASE = "baseUrl"
    BASE_VAL = workspace_rel_path()
    if OPTS not in kwargs[CONFIG]:
        kwargs[CONFIG][OPTS] = {
            BASE: BASE_VAL,
        }
    elif BASE not in kwargs[CONFIG][OPTS]:
        kwargs[CONFIG][OPTS][BASE] = BASE_VAL
    else:
        fail("ts_project.%s.%s.%s must not be set" % (CONFIG, OPTS, BASE))

    PATHS = "paths"
    if PATHS not in kwargs[CONFIG][OPTS]:
        kwargs[CONFIG][OPTS][PATHS] = {"@proof/*": ["./*"]}
    else:
        fail("ts_project.%s.%s.%s must not be set" % (CONFIG, OPTS, PATHS))

    _ts_project(**kwargs)

def _proto_descriptor_impl(ctx):
    return DefaultInfo(files = depset([ctx.attr.proto[ProtoInfo].direct_descriptor_set]))

proto_descriptor = rule(
    doc = """Provider Adapter from ProtoInfo to DefaultInfo.
    Extracts the direct_descriptor_set from the ProtoInfo provided by the proto attr.
    This allows a build target to generate additional source code not done by pbjs/pbts.
    """,
    implementation = _proto_descriptor_impl,
    attrs = {"proto": attr.label(providers = [ProtoInfo])},
)
