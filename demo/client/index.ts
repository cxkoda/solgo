import * as grpc from "@grpc/grpc-js";
import * as commander from "commander";
import * as proto from "@proof/demo/proto/demo_ts_proto.pb";
import demo = proto.proof.demo.v1;
import * as demogrpc from "@proof/demo/proto/demo_ts_proto.grpc";

const program = new commander.Command();

program
  .option(
    "--server_addr <string>",
    "Address of running gRPC server",
    "localhost:8080"
  )
  .option(
    "--echo_payload <string>",
    "Payload to echo via gRPC+EVM",
    "Hello world"
  )
  .addOption(
    new commander.Option(
      "--count_to <number>",
      "End value for server-stream counting"
    )
      .default(10)
      .argParser<number>((val: string, prev: number): number => {
        let num: number = parseInt(val);
        if (isNaN(num)) {
          throw new commander.InvalidArgumentError("not a number");
        }
        return num;
      })
  );

program.parse();
const opts = program.opts();

const client = new demogrpc.proof.demo.v1.ProofDemoServiceClient(
  opts.server_addr,
  grpc.credentials.createInsecure()
);

(async () => {
  await new Promise<void>((resolve, reject) => {
    const echoReq = new demo.EchoRequest({ payload: opts.echo_payload });
    client
      .Echo(echoReq)
      .then((resp: demo.EchoResponse) => {
        process.stdout.write(resp.payload + "\n");
        resolve();
      })
      .catch(reject);
  });

  const stream = client.Count(new demo.CountRequest({ end: opts.count_to }));
  // The returned stream extends a standard Node ReadableStream.
  stream.onData((resp: demo.CountResponse) => console.info(resp.number));
  stream.onEnd(() => console.info("finished counting"));
  stream.onError(console.error);
})();
