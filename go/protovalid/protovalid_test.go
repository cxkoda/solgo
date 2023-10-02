package protovalid

import (
	"google.golang.org/grpc"
)

// TODO(arran) https://linear.app/proofxyz/issue/ENG-699/full-tests-of-protovalid-go-package

// Compile-time type checks.
var (
	_ grpc.UnaryServerInterceptor = UnaryServerInterceptor
	_ grpc.UnaryClientInterceptor = UnaryClientInterceptor
)
