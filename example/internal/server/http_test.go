package server_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/genproto/googleapis/api/httpbody"

	"github.com/aqcool/protoc-gen-go-fiber/binding"
	v1 "github.com/aqcool/protoc-gen-go-fiber/example/api/helloworld/v1"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/server"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/service"
)

func startApp(t *testing.T) (baseURL string, shutdown func()) {
	t.Helper()
	app := server.NewHTTPServer(service.NewGreeterService())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	return "http://" + ln.Addr().String(), func() { _ = app.Shutdown() }
}

func TestGreeterHTTPServer(t *testing.T) {
	app := server.NewHTTPServer(service.NewGreeterService())

	t.Run("SayHello", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/helloworld/cursor", nil)
		resp, err := app.Test(req, fiber.TestConfig{})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "cursor") {
			t.Fatalf("body=%s", body)
		}
	})

	t.Run("CreateHello", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/helloworld", strings.NewReader(`{"name":"post"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, fiber.TestConfig{})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "post") {
			t.Fatalf("body=%s", body)
		}
	})

	t.Run("ListHello", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/helloworld/stream/sse", nil)
		req.Header.Set("Accept", "text/event-stream")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		scanner := bufio.NewScanner(resp.Body)
		var data string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				break
			}
		}
		if !strings.Contains(data, "sse") {
			t.Fatalf("sse data=%q", data)
		}
	})

	t.Run("UploadHello", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/helloworld/upload", strings.NewReader("raw-bytes"))
		req.Header.Set("Content-Type", "text/plain")
		resp, err := app.Test(req, fiber.TestConfig{})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		if string(body) != "raw-bytes" {
			t.Fatalf("body=%q", body)
		}
	})
}

func TestGreeterHTTPClient(t *testing.T) {
	baseURL, shutdown := startApp(t)
	defer shutdown()

	client := v1.NewGreeterHTTPClient(binding.NewHTTPClient(http.DefaultClient, baseURL))

	reply, err := client.SayHello(context.Background(), &v1.HelloRequest{Name: "client"})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetMessage() != "hello client" {
		t.Fatalf("reply=%q", reply.GetMessage())
	}

	chat, err := client.ChatHello(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Close()
	if err := chat.Send(&v1.HelloRequest{Name: "peer"}); err != nil {
		t.Fatal(err)
	}
	chatReply, err := chat.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if chatReply.GetMessage() != "chat peer" {
		t.Fatalf("chat reply=%q", chatReply.GetMessage())
	}

	uploadReply, err := client.UploadHello(context.Background(), &v1.UploadHelloRequest{
		Body: &httpbody.HttpBody{
			ContentType: "text/plain",
			Data:        []byte("upload"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(uploadReply.GetBody().GetData()) != "upload" {
		t.Fatalf("upload body=%q", uploadReply.GetBody().GetData())
	}
}
