// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.19.3
// source: job.proto

package job

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// TransactionClient is the client API for Transaction service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TransactionClient interface {
	Submit(ctx context.Context, in *Request, opts ...grpc.CallOption) (Transaction_SubmitClient, error)
	Update(ctx context.Context, opts ...grpc.CallOption) (Transaction_UpdateClient, error)
}

type transactionClient struct {
	cc grpc.ClientConnInterface
}

func NewTransactionClient(cc grpc.ClientConnInterface) TransactionClient {
	return &transactionClient{cc}
}

func (c *transactionClient) Submit(ctx context.Context, in *Request, opts ...grpc.CallOption) (Transaction_SubmitClient, error) {
	stream, err := c.cc.NewStream(ctx, &Transaction_ServiceDesc.Streams[0], "/job.Transaction/Submit", opts...)
	if err != nil {
		return nil, err
	}
	x := &transactionSubmitClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Transaction_SubmitClient interface {
	Recv() (*Response, error)
	grpc.ClientStream
}

type transactionSubmitClient struct {
	grpc.ClientStream
}

func (x *transactionSubmitClient) Recv() (*Response, error) {
	m := new(Response)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *transactionClient) Update(ctx context.Context, opts ...grpc.CallOption) (Transaction_UpdateClient, error) {
	stream, err := c.cc.NewStream(ctx, &Transaction_ServiceDesc.Streams[1], "/job.Transaction/Update", opts...)
	if err != nil {
		return nil, err
	}
	x := &transactionUpdateClient{stream}
	return x, nil
}

type Transaction_UpdateClient interface {
	Send(*UpdateReceipt) error
	Recv() (*UpdateReceipt, error)
	grpc.ClientStream
}

type transactionUpdateClient struct {
	grpc.ClientStream
}

func (x *transactionUpdateClient) Send(m *UpdateReceipt) error {
	return x.ClientStream.SendMsg(m)
}

func (x *transactionUpdateClient) Recv() (*UpdateReceipt, error) {
	m := new(UpdateReceipt)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TransactionServer is the server API for Transaction service.
// All implementations must embed UnimplementedTransactionServer
// for forward compatibility
type TransactionServer interface {
	Submit(*Request, Transaction_SubmitServer) error
	Update(Transaction_UpdateServer) error
	mustEmbedUnimplementedTransactionServer()
}

// UnimplementedTransactionServer must be embedded to have forward compatible implementations.
type UnimplementedTransactionServer struct {
}

func (UnimplementedTransactionServer) Submit(*Request, Transaction_SubmitServer) error {
	return status.Errorf(codes.Unimplemented, "method Submit not implemented")
}
func (UnimplementedTransactionServer) Update(Transaction_UpdateServer) error {
	return status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedTransactionServer) mustEmbedUnimplementedTransactionServer() {}

// UnsafeTransactionServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TransactionServer will
// result in compilation errors.
type UnsafeTransactionServer interface {
	mustEmbedUnimplementedTransactionServer()
}

func RegisterTransactionServer(s grpc.ServiceRegistrar, srv TransactionServer) {
	s.RegisterService(&Transaction_ServiceDesc, srv)
}

func _Transaction_Submit_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Request)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TransactionServer).Submit(m, &transactionSubmitServer{stream})
}

type Transaction_SubmitServer interface {
	Send(*Response) error
	grpc.ServerStream
}

type transactionSubmitServer struct {
	grpc.ServerStream
}

func (x *transactionSubmitServer) Send(m *Response) error {
	return x.ServerStream.SendMsg(m)
}

func _Transaction_Update_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TransactionServer).Update(&transactionUpdateServer{stream})
}

type Transaction_UpdateServer interface {
	Send(*UpdateReceipt) error
	Recv() (*UpdateReceipt, error)
	grpc.ServerStream
}

type transactionUpdateServer struct {
	grpc.ServerStream
}

func (x *transactionUpdateServer) Send(m *UpdateReceipt) error {
	return x.ServerStream.SendMsg(m)
}

func (x *transactionUpdateServer) Recv() (*UpdateReceipt, error) {
	m := new(UpdateReceipt)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Transaction_ServiceDesc is the grpc.ServiceDesc for Transaction service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Transaction_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "job.Transaction",
	HandlerType: (*TransactionServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Submit",
			Handler:       _Transaction_Submit_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Update",
			Handler:       _Transaction_Update_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "job.proto",
}
