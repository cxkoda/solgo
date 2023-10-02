import * as grpc from "@grpc/grpc-js";

/**
 * Adds strict typing to grpc.ClientReadableStream events, removing the on()
 * method in lieu of on[Event]() methods for each of Data, Error, End, Status,
 * and Metadata.
 */
export class ClientReadableStream<T> {
  private stream: grpc.ClientReadableStream<T>;

  /**
   * Constructs a new ClientReadableStream.
   * @param stream The underlying grpc.ClientReadableStream from which events
   * are read.
   */
  constructor(stream: grpc.ClientReadableStream<T>) {
    this.stream = stream;
  }

  public onData(cb: (resp: T) => void) {
    this.stream.on("data", cb);
  }

  public onError(cb: (err: Error) => void) {
    this.stream.on("error", cb);
  }

  public onEnd(cb: () => void) {
    this.stream.on("end", cb);
  }

  public onMetadata(cb: (md: grpc.Metadata) => void) {
    this.stream.on("metadata", cb);
  }

  public onStatus(cb: (status: grpc.StatusObject) => void) {
    this.stream.on("status", cb);
  }

  /**
   * Cancel the ongoing call. Results in the call ending with a CANCELLED
   * status, unless it has already ended with some other status.
   */
  public cancel(): void {
    this.stream.cancel();
  }

  /**
   * Get the endpoint this stream is connected to.
   * 
   * @returns The URI of the endpoint.
   */
  public getPeer(): string {
    return this.stream.getPeer();
  }
}
