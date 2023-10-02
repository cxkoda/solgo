// Package protovalid provides gRPC interceptors for validating protobuf
// messages compiled with protoc-gen-validate.
package protovalid

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validate returns m.ValidateAll() if m is a proto message with additional
// validation methods, otherwise it returns nil. A non-nil return value will be
// a status error of the specified code.
func validate(m interface{}, errCode codes.Code) error {
	type validator interface {
		proto.Message
		Validate() error
		ValidateAll() error
	}

	v, ok := m.(validator)
	if !ok {
		return nil
	}
	if err := v.ValidateAll(); err != nil {
		return status.Errorf(errCode, err.Error())
	}
	return nil
}

// UnaryServerInterceptor is a grpc.UnaryServerInterceptor that validates both
// the client request and the server response. An invalid request results in
// codes.InvalidArgument whereas an invalid response results in codes.Internal.
func UnaryServerInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (interface{}, error) {
	if err := validate(req, codes.InvalidArgument); err != nil {
		return nil, err
	}

	reply, err := h(ctx, req)
	if err != nil {
		return nil, err
	}
	// Note: the use of codes.Internal is _not_ because it is internal to the
	// system, but because an invalid reply suggests a bug that needs attention.
	// See codes.Code documentation for explanation of each.
	if err := validate(reply, codes.Internal); err != nil {
		return nil, err
	}
	return reply, nil
}

// UnaryClientInterceptor is a grpc.UnaryClientInterceptor that validates the
// request only, returning a codes.InvalidArgument error should validation fail.
func UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if err := validate(req, codes.InvalidArgument); err != nil {
		return err
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
