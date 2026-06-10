package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"google.golang.org/genproto/googleapis/api/httpbody"

	"github.com/aqcool/protoc-gen-go-fiber/binding"
	v1 "github.com/aqcool/protoc-gen-go-fiber/example/api/helloworld/v1"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/conf"
)

func main() {
	confPath := flag.String("conf", "configs/config.yaml", "config file path")
	flag.Parse()

	bc, err := conf.Load(*confPath)
	if err != nil {
		log.Fatal(err)
	}

	client := v1.NewGreeterHTTPClient(binding.NewHTTPClient(http.DefaultClient, bc.Client.HTTP.Endpoint))
	ctx := context.Background()

	reply, err := client.SayHello(ctx, &v1.HelloRequest{Name: "fiber"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("SayHello:", reply.GetMessage())

	reply, err = client.CreateHello(ctx, &v1.HelloRequest{Name: "world"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("CreateHello:", reply.GetMessage())

	stream, err := client.ListHello(ctx, &v1.HelloRequest{Name: "fiber"})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()
	for i := 0; i < 3; i++ {
		msg, err := stream.Recv()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("ListHello:", msg.GetMessage())
	}

	chat, err := client.ChatHello(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer chat.Close()
	if err := chat.Send(&v1.HelloRequest{Name: "ws"}); err != nil {
		log.Fatal(err)
	}
	chatReply, err := chat.Recv()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ChatHello:", chatReply.GetMessage())

	uploadReply, err := client.UploadHello(ctx, &v1.UploadHelloRequest{
		Body: &httpbody.HttpBody{
			ContentType: "text/plain",
			Data:        []byte("payload"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("UploadHello:", string(uploadReply.GetBody().GetData()))
}
