package service_test

import (
	"context"
	"testing"

	v1 "github.com/aqcool/protoc-gen-go-fiber/example/api/helloworld/v1"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/service"
)

func TestGreeterService_SayHello(t *testing.T) {
	svc := service.NewGreeterService()
	reply, err := svc.SayHello(context.Background(), &v1.HelloRequest{Name: "kratos"})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetMessage() != "hello kratos" {
		t.Fatalf("message=%q", reply.GetMessage())
	}
}
