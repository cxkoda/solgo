package grpctest

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/h-fam/errdiff"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/proofxyz/solgo/go/grpctest/proto"
)

type echo struct{}

func (echo) Echo(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	return &pb.Response{
		Msg: in.Msg,
	}, nil
}

func ExampleTester() {
	st := New()
	defer st.Close()

	// Register as many services as necessary. If only one is needed, rather use
	// NewClientConn() which handles all of the additional steps, and accepts
	// identical arguments to RegisterService().

	// This variable declaration is purely demonstrative of the fact that
	// echo{} implements the interface.
	var svc pb.EchoServiceServer = &echo{}
	if err := st.RegisterService(pb.RegisterEchoServiceServer, svc); err != nil {
		fmt.Printf("RegisterService() got err %v", err)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		st.Serve()
	}()

	conn, err := st.Dial()
	if err != nil {
		fmt.Printf("%T.Dial() got err %v", st, err)
		return
	}

	c := pb.NewEchoServiceClient(conn)
	resp, err := c.Echo(context.Background(), &pb.Request{Msg: "hello world"})
	if err != nil {
		fmt.Printf("EchoService.Echo() got err %v", err)
		return
	}
	fmt.Println(resp.Msg)

	st.Close()
	// Not entirely necessary; but demonstrates that the Server has stopped.
	<-done

	// Output: hello world
}

func ExampleNewClientConn() {
	// This variable declaration is purely demonstrative of the fact that
	// echo{} implements the interface.
	var svc pb.EchoServiceServer = &echo{}
	conn, cleanup, err := NewClientConn(pb.RegisterEchoServiceServer, svc)
	if err != nil {
		fmt.Printf("NewClientConn() got err %v; want nil err", err)
		return
	}
	defer cleanup()

	c := pb.NewEchoServiceClient(conn)
	resp, err := c.Echo(context.Background(), &pb.Request{Msg: "foobar"})
	if err != nil {
		fmt.Printf("EchoService.Echo() got err %v; want nil err", err)
	}
	fmt.Println(resp.Msg)

	// Output: foobar
}

func TestServerOpts(t *testing.T) {
	// Test that ServerOptions are propagated by creating an interceptor that
	// adds a suffix to the message.
	const suffix = " world"
	uiOpt := grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}
		switch r := resp.(type) {
		case *pb.Response:
			return &pb.Response{Msg: fmt.Sprintf("%s%s", r.Msg, suffix)}, nil
		default:
			return nil, status.Errorf(codes.Internal, "unexpected response type %T", r)
		}
	})

	conn := NewClientConnTB[pb.EchoServiceServer](t, pb.RegisterEchoServiceServer, &echo{}, uiOpt)
	c := pb.NewEchoServiceClient(conn)
	got, err := c.Echo(context.Background(), &pb.Request{Msg: "hello"})
	if err != nil {
		t.Errorf("EchoService.Echo() with interceptor got err %v; want nil err", err)
	}

	want := &pb.Response{
		Msg: "hello world",
	}
	if !proto.Equal(got, want) {
		t.Errorf("EchoService.Echo(hello) with suffix interceptor; got %v; want %v", got, want)
	}
}

func TestRegisterErrors(t *testing.T) {
	tests := []struct {
		name               string
		fn, implementation interface{}

		// want is a string that must be contained in the error
		// message.
		want string
	}{
		{
			name:           "nil register function",
			fn:             nil,
			implementation: &echo{},
			want:           "nil",
		},
		{
			name:           "function-typed-nil register function",
			fn:             (func())(nil),
			implementation: &echo{},
			want:           "nil",
		},
		{
			name:           "nil implementation",
			fn:             pb.RegisterEchoServiceServer,
			implementation: nil,
			want:           "nil",
		},
		{
			name:           "typed-nil implementation",
			fn:             pb.RegisterEchoServiceServer,
			implementation: (*echo)(nil),
			want:           "nil",
		},
		{
			name:           "incorrect register-function kind",
			fn:             "hello",
			implementation: &echo{},
			want:           "function",
		},
		{
			name:           "register function num in",
			fn:             func(_, _, _ string) {},
			implementation: &echo{},
			want:           "input",
		},
		{
			name:           "register function num out",
			fn:             func(_, _ string) error { return nil },
			implementation: &echo{},
			want:           "returns",
		},
		{
			name:           "register function first parameter not gRPC server",
			fn:             func(_ string, _ pb.EchoServiceServer) {},
			implementation: &echo{},
			want:           "*grpc.Server",
		},
		{
			name:           "register function second parameter not service interface",
			fn:             func(_ *grpc.Server, _ string) {},
			implementation: &echo{},
			want:           reflect.TypeOf(&echo{}).String(),
		},
		{
			name:           "bad service implementation",
			fn:             pb.RegisterEchoServiceServer,
			implementation: struct{}{},
			want:           "struct",
		},
		{
			name: "panic recovery",
			fn: func(_ *grpc.Server, _ pb.EchoServiceServer) {
				panic("don't")
			},
			implementation: &echo{},
			want:           "panic: don't",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := New()
			defer tester.Close()

			if diff := errdiff.Substring(tester.RegisterService(tt.fn, tt.implementation), tt.want); diff != "" {
				t.Errorf("%T.RegisterService() %s", tester, diff)
			}
		})
	}
}
