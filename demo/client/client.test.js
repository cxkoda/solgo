const { credentials } = require("@grpc/grpc-js");
const { spawn } = require("node:child_process");
const { runfiles } = require("@bazel/runfiles");
const grpc = require("../proto/demo_ts_proto.grpc");

describe("the demo gRPC client", () => {
  var portNumber;
  var server;
  var client;
  var serverClosed;

  beforeAll(async () => {
    const bin = runfiles.resolve("proof/demo/server/server_/server");
    portNumber = Math.round(Math.random() * 10000 + 1000);
    server = spawn(bin, ["--logtostderr", "--port", portNumber]);

    // Unblocks when the server's logs indicate that it's listening.
    // TODO(aschlosberg) abstract this setup into a shared library.
    await new Promise((resolve) => {
      server.stderr.on("data", (data) => {
        const str = data.toString();
        console.log(str);
        if (str.search(`Listening on ":${portNumber}"`) != -1) {
          resolve();
        }
      });
    });

    serverClosed = new Promise((resolve) => {
      server.on("close", resolve);
    });

    client = new grpc.proof.demo.v1.ProofDemoServiceClient(
      `localhost:${portNumber}`,
      credentials.createInsecure()
    );
  });

  it("propagates echoes through multiple languages", async () => {
    const promises = ["hello", "world", "foo"].map(async (payload) => {
      const req = { payload: payload };
      const resp = await client.Echo(req);
      expect(resp.payload).toBe(
        `gRPC: Go: Solidity: ${payload}`
      );
    });

    await Promise.all(promises);
  });

  it("receives variable-length server streams", async () => {
    const tests = {
      0: [],
      3: [0, 1, 2],
      10: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
    }

    const promises = Object.keys(tests).map(async (end) => {
      return new Promise((resolve, reject) => {
        const stream = client.Count({ end: end });
        stream.onError(reject);

        var received = [];
        stream.onData((resp) => received.push(resp.number));

        stream.onEnd(() => {
          expect(received).toStrictEqual(tests[end]);
          resolve();
        });
      })
    });

    await Promise.all(promises);
  });

  afterAll(async () => {
    server.kill("SIGINT");
    await serverClosed;
  });
});
