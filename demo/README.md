# Runnable examples

The code in this directory demonstrates integrated usage of regular Bazel rules
along with PROOF-specific custom extensions. It is built and tested with regular
CI, so is guaranteed to function correctly.

## Usage

### Server

Start a gRPC server written in Go. The first `--` indicates to Bazel that all
subsequent arguments must be forwarded to the target being run, not interpreted
by Bazel itself.

`bazel run //demo/server -- --logtostderr`

This server depends on protobuf definitions in `./proto` and Solidity code in
`./contract`. It deploys the contract to an in-process blockchain.

### Client

In a new terminal, run a gRPC client written in Typescript.

`bazel run //demo/client -- --echo_payload=Foo --count_to=10`

The echo payload is sent to the server over gRPC, which propagates it to the
deployed contract. Each element tags the payload before echoing it. The
`--count_to` flag demonstrates streaming gRPC, receiving that many messages from
the server.

This client similarly depends on the protobuf definitions, specifically a
`ts_proto_library()` target called `demo_ts_proto`. This allows `index.ts` to
import `demo_ts_proto.pb` for protobuf bindings and `demo_ts_proto.grpc` for
creation of a gRPC client that depends on the `.pb` types.

## Testing

`bazel test //demo/...`

The test depends on the compiled server binary, made available by Bazel via a
`data` attribute. This allows for full end-to-end testing as a `beforeAll` Jest
hook deploys a fresh server binary that is later cleaned up in the `afterAll`
function.