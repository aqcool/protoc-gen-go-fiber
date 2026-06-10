// Package service 实现 proto 定义的 GreeterHTTPServer 接口（业务逻辑层）。
package service

import (
	"context"

	"google.golang.org/genproto/googleapis/api/httpbody"

	v1 "github.com/aqcool/protoc-gen-go-fiber/example/api/helloworld/v1"
)

// GreeterService implements helloworld.v1.GreeterHTTPServer.
type GreeterService struct{}

func NewGreeterService() *GreeterService {
	return &GreeterService{}
}

func (s *GreeterService) SayHello(_ context.Context, req *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: "hello " + req.GetName()}, nil
}

func (s *GreeterService) CreateHello(_ context.Context, req *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: "created " + req.GetName()}, nil
}

func (s *GreeterService) ListHello(req *v1.HelloRequest, stream v1.Greeter_ListHelloServer) error {
	for _, suffix := range []string{"one", "two", "three"} {
		if err := stream.Send(&v1.HelloReply{Message: "stream " + req.GetName() + " " + suffix}); err != nil {
			return err
		}
	}
	return nil
}

func (s *GreeterService) ChatHello(stream v1.Greeter_ChatHelloServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := stream.Send(&v1.HelloReply{Message: "chat " + req.GetName()}); err != nil {
			return err
		}
	}
}

func (s *GreeterService) UploadHello(_ context.Context, req *v1.UploadHelloRequest) (*v1.UploadHelloReply, error) {
	return &v1.UploadHelloReply{
		Body: &httpbody.HttpBody{
			ContentType: req.GetBody().GetContentType(),
			Data:        append([]byte(nil), req.GetBody().GetData()...),
		},
	}, nil
}
