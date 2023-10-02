package demo

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/facebookgo/freeport"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	port, err := freeport.Get()
	if err != nil {
		t.Fatalf("freeport.Get() error %v", err)
	}
	t.Logf("Launching server on port %d", port)

	server := exec.CommandContext(
		ctx,
		findBinary(t, "demo/server", "server"),
		fmt.Sprintf("--port=%d", port),
		"--logtostderr",
	)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	server.Stdout = stdout
	server.Stderr = stderr
	t.Cleanup(func() {
		t.Logf("server stdout:\n%s", stdout.String())
		t.Logf("server stderr:\n%s", stderr.String())
	})

	if err := server.Start(); err != nil {
		t.Fatalf("%T(server).Start() error %v", server, err)
	}
	waitForGRPCServer(t, fmt.Sprintf("localhost:%d", port))

	tests := []struct {
		echo    string
		countTo int
		want    string
	}{
		{
			echo:    "hello",
			countTo: 3,
			want: `gRPC: Go: Solidity: "hello"
0
1
2
finished counting
`,
		},
		{
			echo:    "world",
			countTo: 5,
			want: `gRPC: Go: Solidity: "world"
0
1
2
3
4
finished counting
`,
		},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("echo %q; count %d", tt.echo, tt.countTo)
		t.Run(name, func(t *testing.T) {
			client := exec.Command(
				// js_binary creates a shell script that launches node with the
				// entry point, so we don't need to manage any of that.
				runFile(t, "demo/client/client.sh"),
				fmt.Sprintf("--server_addr=localhost:%d", port),
				fmt.Sprintf("--echo_payload=%q", tt.echo),
				fmt.Sprintf("--count_to=%d", tt.countTo),
			)

			got, err := client.CombinedOutput()
			if err != nil {
				t.Fatalf("%T(client).CombinedOutput() error %v", client, err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("%T(client).CombinedOutput() diff (-want +got): \n%s", client, diff)
			}
		})
	}

	cancel()
	server.Wait()
}

// findBinary returns bazel.FindBinary(pkg, name) or reports on tb.Fatal if
// not found.
func findBinary(tb testing.TB, pkg, name string) string {
	tb.Helper()
	bin, ok := bazel.FindBinary(pkg, name)
	if !ok {
		tb.Fatalf("bazel.FindBinary(%q, %q) returned false", pkg, name)
	}
	return bin
}

// runFile returns bazel.Runfile(path) or reports any error on tb.Fatal.
func runFile(tb testing.TB, path string) string {
	tb.Helper()
	f, err := bazel.Runfile(path)
	if err != nil {
		tb.Fatalf("bazel.Runfile(%q) error %v", path, err)
	}
	return f
}

// waitForGRPCServer performs a blocking dial to the target, reporting any error
// on tb.Fatal. When this function returns, the server is guaranteed to have
// started listening for gRPC requests.
func waitForGRPCServer(tb testing.TB, target string) {
	tb.Helper()
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		tb.Fatalf("grpc.Dial(%q, WithBlock, WithTransportCreds(insecure)) error %v", target, err)
	}
	if err := conn.Close(); err != nil {
		tb.Errorf("%T.Close() error %v", conn, err)
	}
}
