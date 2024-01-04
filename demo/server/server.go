// The server binary implements a demo gRPC server that runs a simulated EVM
// blockchain used for echoing responses.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/divergencetech/ethier/ethtest"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/cxkoda/solgo/demo/contract"
	pb "github.com/cxkoda/solgo/demo/proto"
)

func main() {
	port := flag.Int("port", 8080, "Port on which to listen for gRPC requests")
	flag.Parse()

	d, err := newDemo()
	if err != nil {
		glog.Exitf("Creating demo: %v", err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(intercept))
	pb.RegisterProofDemoServiceServer(s, d)
	reflection.Register(s)

	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		glog.Exitf("net.Listen(tcp, %q): %v", addr, err)
	}
	glog.Infof("Listening on %q", addr)

	if err := s.Serve(lis); err != nil {
		glog.Exitf("%T.Serve(): %v", s, err)
	}
}

// newDemo deploys a Demo contract on a simulated blockchain and returns a
// ProofDemoServiceServer with its Echo method backed by the deployed contract.
func newDemo() (pb.ProofDemoServiceServer, error) {
	sim, err := ethtest.NewSimulatedBackend(1)
	if err != nil {
		return nil, fmt.Errorf("ethtest.NewSimulatedBackend(): %v", err)
	}
	_, _, d, err := contract.DeployDemo(sim.Acc(0), sim)
	if err != nil {
		return nil, fmt.Errorf("DeployDemo(): %v", err)
	}

	return &demo{
		contract: d,
	}, nil
}

// demo implements the ProofDemoServiceServer.
type demo struct {
	contract *contract.Demo
}

// Echo echoes the request payload, via the simulated contract.
func (d *demo) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	resp, err := d.contract.Echo(nil, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("%T.Echo(nil, %q): %v", d.contract, req.Payload, err)
	}

	return &pb.EchoResponse{
		Payload: fmt.Sprintf("Go: %s", resp),
	}, nil
}

// Abi returns the Demo contract ABI.
func (*demo) Abi(ctx context.Context, req *pb.AbiRequest) (*pb.AbiResponse, error) {
	return &pb.AbiResponse{
		Abi: contract.DemoMetaData.ABI,
	}, nil
}

// Count returns a stream of numbers.
func (*demo) Count(req *pb.CountRequest, srv pb.ProofDemoService_CountServer) error {
	if req.End > 16 {
		return status.Errorf(codes.InvalidArgument, "end %d > 16", req.End)
	}
	for i := uint32(0); i < req.End; i++ {
		resp := &pb.CountResponse{Number: i}
		if err := srv.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

// intercept is a grpc.UnaryServerInterceptor that prefixes EchoResponse
// payloads to demonstrate that the full gRPC stack has been used.
func intercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}

	if e, ok := resp.(*pb.EchoResponse); ok {
		e.Payload = fmt.Sprintf("gRPC: %s", e.Payload)
	}
	return resp, nil
}
