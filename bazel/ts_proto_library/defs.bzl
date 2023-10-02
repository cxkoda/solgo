"""pbjs+pbts support with generation of gRPC service definitions.

Original licensed under Apache 2.0, Copyright Aspect Build Systems Inc
Source: https://github.com/aspect-build/bazel-examples/blob/64a5066b4be2e85b0a0a0e4a281ee185c75dd67a/protobufjs/defs.bzl
"""

load("//bazel:defs.bzl", "proto_descriptor", "ts_project", "workspace_rel_path")
load("@npm//:protobufjs-cli/package_json.bzl", "bin")
load("@rules_proto//proto:defs.bzl", "ProtoInfo")

def _proto_sources_impl(ctx):
    return DefaultInfo(files = ctx.attr.proto[ProtoInfo].transitive_sources)

_proto_sources = rule(
    doc = """Provider Adapter from ProtoInfo to DefaultInfo.
        Extracts the transitive_sources from the ProtoInfo provided by the proto attr.
        This allows a macro to access the complete set of .proto files needed during compilation.
        """,
    implementation = _proto_sources_impl,
    attrs = {"proto": attr.label(providers = [ProtoInfo])},
)

def ts_proto_library(name, proto, deps = [], **kwargs):
    """Minimal wrapper macro around pbjs/pbts tooling + gRPC client generation.

    A single proto_library is used as input into the pbjs executable to generate
    JavaScript bindings for the protobuf types, based on protobufjs. The pbjs
    output is piped to pbts to create accompanying .d.ts definitions. Together
    these result in creation of ${name}.pb.js and ${name}.pb.d.ts files that can
    be imported as 'path/to/${name}.pb'.

    A custom code generator, grpc_gen_ts, outputs ${name}.grpc.ts with gRPC
    service definitions defined in the proto, and respective client classes.
    These can be imported as 'path/to/${name}.grpc' and accept the corresponding
    .pb types as request/response protos.

    See https://www.npmjs.com/package/protobufjs-cli re pbjs and pbts.

    Args:
        name: name of generated ts_project target
        proto: label of a single proto_library target to generate for
        deps: additional dependencies of the ts_project
        **kwargs: passed through to the ts_project.
    """

    js_out = name + ".pb.js"
    ts_out = name + ".pb.d.ts"
    grpc_out = name + ".grpc.ts"

    # Generate some target names, based on the provided name
    # (so that they are unique if the macro is called several times in one package)
    proto_sources_target = "_%s_proto_sources" % name
    proto_descriptor_target = "_%s_proto_descriptor" % name
    js_target = "_%s_pbjs" % name
    ts_target = "_%s_pbts" % name
    grpc_target = "_%s_grpc" % name

    # grab the transitive .proto files needed to compile the given one
    _proto_sources(
        name = proto_sources_target,
        proto = proto,
    )

    # Transform .proto files to a single _pb.js file named after the macro
    bin.pbjs(
        name = js_target,
        srcs = [":" + proto_sources_target],
        copy_srcs_to_bin = False,
        chdir = "../../../",
        # Arguments documented at
        # https://github.com/protobufjs/protobuf.js/tree/6.8.8#pbjs-for-javascript
        args = [
            "--target=static-module",
            "--root=%s" % name,
            "--out=$@",
            "--no-service",  # we use grpc-js directly
            "$(locations %s)" % proto_sources_target,
        ],
        outs = [js_out],
    )

    # Transform the _pb.js file to a .d.ts file with TypeScript types
    bin.pbts(
        name = ts_target,
        srcs = [js_target],
        copy_srcs_to_bin = False,
        chdir = "../../../",
        # Arguments documented at
        # https://github.com/protobufjs/protobuf.js/tree/6.8.8#pbts-for-typescript
        args = [
            "--out=$@",
            "$(execpath %s)" % js_target,
        ],
        outs = [ts_out],
    )

    # Extract the FileDescriptorProto of the proto being compiled, not its deps.
    proto_descriptor(
        name = proto_descriptor_target,
        proto = proto,
    )

    proto_descriptor_label = ":" + proto_descriptor_target

    # Generate Typescript bindings to use gRPC+pbjs together.
    native.genrule(
        name = grpc_target,
        outs = [grpc_out],
        srcs = [proto_descriptor_label],
        tools = [
            "//bazel/ts_proto_library/grpc_gen_ts",
        ],
        cmd = """
        cat $(location %s) | \
        $(location //bazel/ts_proto_library/grpc_gen_ts) \
            --pb_target=%s.pb \
            --workspace_rel_path=%s \
            > $@
        """ % (proto_descriptor_label, name, workspace_rel_path()),
    )

    ts_project(
        name = name,
        srcs = [js_target, ts_target, grpc_target],
        tsconfig = {},
        declaration = True,
        deps = deps + [
            "//:node_modules/long",
            "//:node_modules/protobufjs",
            "//:node_modules/@types/node",
            "//:node_modules/@grpc/grpc-js",
            "//typescript:grpc",
        ],
        **kwargs
    )
