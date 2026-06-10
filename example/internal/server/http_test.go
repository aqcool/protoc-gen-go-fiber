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

func newClient(t *testing.T, baseURL string) v1.GreeterHTTPClient {
	t.Helper()
	return v1.NewGreeterHTTPClient(binding.NewHTTPClient(http.DefaultClient, baseURL))
}

func readSSEFrames(t *testing.T, r io.Reader) []string {
	t.Helper()
	scanner := bufio.NewScanner(r)
	var frames []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			frames = append(frames, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return frames
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
		frames := readSSEFrames(t, resp.Body)
		if len(frames) != 3 {
			t.Fatalf("frame count=%d want 3 frames=%v", len(frames), frames)
		}
		for i, suffix := range []string{"one", "two", "three"} {
			if !strings.Contains(frames[i], "sse") || !strings.Contains(frames[i], suffix) {
				t.Fatalf("frame[%d]=%q", i, frames[i])
			}
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

	// WebSocket 无法通过 app.Test 完成 upgrade，使用真实 Listener 验证 ChatHello。
	t.Run("ChatHello", func(t *testing.T) {
		baseURL, shutdown := startApp(t)
		defer shutdown()

		chat, err := newClient(t, baseURL).ChatHello(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer chat.Close()
		if err := chat.Send(&v1.HelloRequest{Name: "server-ws"}); err != nil {
			t.Fatal(err)
		}
		reply, err := chat.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if reply.GetMessage() != "chat server-ws" {
			t.Fatalf("reply=%q", reply.GetMessage())
		}
	})
}

func TestGreeterHTTPClient(t *testing.T) {
	baseURL, shutdown := startApp(t)
	defer shutdown()
	client := newClient(t, baseURL)
	ctx := context.Background()

	t.Run("SayHello", func(t *testing.T) {
		reply, err := client.SayHello(ctx, &v1.HelloRequest{Name: "client"})
		if err != nil {
			t.Fatal(err)
		}
		if reply.GetMessage() != "hello client" {
			t.Fatalf("reply=%q", reply.GetMessage())
		}
	})

	t.Run("CreateHello", func(t *testing.T) {
		reply, err := client.CreateHello(ctx, &v1.HelloRequest{Name: "world"})
		if err != nil {
			t.Fatal(err)
		}
		if reply.GetMessage() != "created world" {
			t.Fatalf("reply=%q", reply.GetMessage())
		}
	})

	t.Run("ListHello", func(t *testing.T) {
		stream, err := client.ListHello(ctx, &v1.HelloRequest{Name: "fiber"})
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		for i, suffix := range []string{"one", "two", "three"} {
			msg, err := stream.Recv()
			if err != nil {
				t.Fatalf("recv[%d]: %v", i, err)
			}
			if !strings.Contains(msg.GetMessage(), "fiber") || !strings.Contains(msg.GetMessage(), suffix) {
				t.Fatalf("msg[%d]=%q", i, msg.GetMessage())
			}
		}
	})

	t.Run("ChatHello", func(t *testing.T) {
		chat, err := client.ChatHello(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer chat.Close()
		if err := chat.Send(&v1.HelloRequest{Name: "peer"}); err != nil {
			t.Fatal(err)
		}
		reply, err := chat.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if reply.GetMessage() != "chat peer" {
			t.Fatalf("reply=%q", reply.GetMessage())
		}
	})

	t.Run("UploadHello", func(t *testing.T) {
		reply, err := client.UploadHello(ctx, &v1.UploadHelloRequest{
			Body: &httpbody.HttpBody{
				ContentType: "text/plain",
				Data:        []byte("upload"),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if string(reply.GetBody().GetData()) != "upload" {
			t.Fatalf("body=%q", reply.GetBody().GetData())
		}
	})
}
