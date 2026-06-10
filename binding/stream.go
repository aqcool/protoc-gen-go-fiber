package binding

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	fws "github.com/gofiber/contrib/v3/websocket"
	fwsock "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	ssemw "github.com/gofiber/fiber/v3/middleware/sse"
	"google.golang.org/protobuf/proto"
)

// ServerStream sends proto messages over SSE.
type ServerStream struct {
	stream *ssemw.Stream
}

// NewServerStream 包装 Fiber SSE 中间件，每条 event 的 data 为一帧 protojson。
func NewServerStream(handler func(c fiber.Ctx, stream *ServerStream) error) fiber.Handler {
	return ssemw.New(ssemw.Config{
		Handler: func(c fiber.Ctx, stream *ssemw.Stream) error {
			return handler(c, &ServerStream{stream: stream})
		},
	})
}

func (s *ServerStream) Context() context.Context { return s.stream.Context() }

func (s *ServerStream) Send(msg proto.Message) error {
	data, err := marshaler.Marshal(msg)
	if err != nil {
		return err
	}
	return s.stream.Event(ssemw.Event{Data: string(data)})
}

// WebSocketStream adapts websocket connections for proto frame IO.
type WebSocketStream struct {
	conn *fws.Conn
}

// NewWebSocketHandler 包装 contrib/websocket，每条 message 为一帧 protojson（与 Kratos 控制帧不互通）。
func NewWebSocketHandler(handler func(conn *WebSocketStream) error, cfg ...fws.Config) fiber.Handler {
	return fws.New(func(conn *fws.Conn) {
		_ = handler(&WebSocketStream{conn: conn})
	}, cfg...)
}

func (s *WebSocketStream) Recv(msg proto.Message) error {
	_, data, err := s.conn.ReadMessage()
	if err != nil {
		return err
	}
	return unmarshaler.Unmarshal(data, msg)
}

func (s *WebSocketStream) Send(msg proto.Message) error {
	data, err := marshaler.Marshal(msg)
	if err != nil {
		return err
	}
	return s.conn.WriteMessage(fwsock.TextMessage, data)
}

func (s *WebSocketStream) Close() error {
	return s.conn.Close()
}

// ClientStream reads server-sent protojson frames.
type ClientStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	cancel context.CancelFunc
}

func (s *ClientStream) Recv(msg proto.Message) error {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		return unmarshaler.Unmarshal([]byte(data), msg)
	}
}

func (s *ClientStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// WSClientStream sends/receives protojson over websocket.
type WSClientStream struct {
	conn *fwsock.Conn
	mu   sync.Mutex
}

func (s *WSClientStream) Send(msg proto.Message) error {
	data, err := marshaler.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(fwsock.TextMessage, data)
}

func (s *WSClientStream) Recv(msg proto.Message) error {
	_, data, err := s.conn.ReadMessage()
	if err != nil {
		return err
	}
	return unmarshaler.Unmarshal(data, msg)
}

func (s *WSClientStream) Close() error {
	return s.conn.Close()
}

// HTTPClient 封装 net/http，供生成客户端发起 unary / SSE / WebSocket 请求。
type HTTPClient struct {
	client   *http.Client
	endpoint string
}

func NewHTTPClient(client *http.Client, endpoint string) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{client: client, endpoint: strings.TrimRight(endpoint, "/")}
}

type callConfig struct {
	headers map[string]string
}

// CallOption configures outbound HTTP calls.
type CallOption func(*callConfig)

func Header(key, value string) CallOption {
	return func(c *callConfig) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[key] = value
	}
}

// Invoke performs a unary HTTP call with protojson payload.
func (hc *HTTPClient) Invoke(ctx context.Context, method, path string, in proto.Message, out any, opts ...CallOption) error {
	cfg := callConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	var body io.Reader
	contentType := "application/json"
	if in != nil {
		if isHTTPBodyMessage(in.ProtoReflect().Descriptor()) {
			contentType = BodyContentType(in)
			dataFD := in.ProtoReflect().Descriptor().Fields().ByName("data")
			body = bytes.NewReader(in.ProtoReflect().Get(dataFD).Bytes())
		} else {
			data, err := marshaler.Marshal(in)
			if err != nil {
				return err
			}
			body = bytes.NewReader(data)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, hc.endpoint+path, body)
	if err != nil {
		return err
	}
	for k, v := range cfg.headers {
		req.Header.Set(k, v)
	}
	if in != nil {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", contentType)
		}
	}
	resp, err := hc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	switch v := out.(type) {
	case proto.Message:
		if isHTTPBodyMessage(v.ProtoReflect().Descriptor()) {
			return setHTTPBody(v, resp.Header.Get("Content-Type"), data)
		}
		return unmarshaler.Unmarshal(data, v)
	default:
		return json.Unmarshal(data, out)
	}
}

// ServerSentEvent opens an SSE stream.
func (hc *HTTPClient) ServerSentEvent(ctx context.Context, method, path string, in proto.Message, opts ...CallOption) (*ClientStream, error) {
	cfg := callConfig{headers: map[string]string{"Accept": "text/event-stream"}}
	for _, o := range opts {
		o(&cfg)
	}
	var body io.Reader
	if in != nil {
		data, err := marshaler.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
		cfg.headers["Content-Type"] = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, hc.endpoint+path, body)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("http: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return &ClientStream{reader: bufio.NewReader(resp.Body), body: resp.Body}, nil
}

// WebSocket dials a websocket endpoint.
func (hc *HTTPClient) WebSocket(ctx context.Context, path string, opts ...CallOption) (*WSClientStream, error) {
	cfg := callConfig{headers: map[string]string{
		"Accept": "application/json",
	}}
	for _, o := range opts {
		o(&cfg)
	}
	url := strings.Replace(hc.endpoint, "http://", "ws://", 1)
	url = strings.Replace(url, "https://", "wss://", 1)
	url += path
	header := http.Header{}
	for k, v := range cfg.headers {
		header.Set(k, v)
	}
	dialer := fwsock.DefaultDialer
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	return &WSClientStream{conn: conn}, nil
}

// DecodeField unmarshals response bytes into out, optionally a sub-field.
func DecodeField(data []byte, out proto.Message, fieldPath string) error {
	if fieldPath == "" {
		return unmarshaler.Unmarshal(data, out)
	}
	if err := unmarshaler.Unmarshal(data, out); err != nil {
		return err
	}
	sub, err := extractMessage(out, fieldPath)
	if err != nil {
		return err
	}
	encoded, err := marshaler.Marshal(sub)
	if err != nil {
		return err
	}
	return unmarshaler.Unmarshal(encoded, out)
}

// EncodeField returns the request body bytes for a message or sub-field.
func EncodeField(in proto.Message, fieldPath string) ([]byte, error) {
	if fieldPath == "" {
		return marshaler.Marshal(in)
	}
	sub, err := extractMessage(in, fieldPath)
	if err != nil {
		return nil, err
	}
	return marshaler.Marshal(sub)
}

// JSONField returns JSON string for debugging.
func JSONField(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
