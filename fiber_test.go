package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestNoParameters(t *testing.T) {
	path := "/test/noparams"
	m := buildPathVars(path)
	if !reflect.DeepEqual(m, map[string]*string{}) {
		t.Fatalf("Map should be empty")
	}
}

func TestSingleParam(t *testing.T) {
	path := "/test/{message.id}"
	m := buildPathVars(path)
	if len(m) != 1 {
		t.Fatalf("len(m) not is 1")
	}
	if m["message.id"] != nil {
		t.Fatalf(`m["message.id"] should be empty`)
	}
}

func TestReplacePath(t *testing.T) {
	path := "/test/{message.id}/{message.name=messages/*}"
	newPath := replacePath("message.name", "messages/*", path)
	if newPath != "/test/{message.id}/{message.name:messages/[^/]+}" {
		t.Fatalf("unexpected path %s", newPath)
	}
}

func TestPathTemplateRegex(t *testing.T) {
	if got := pathTemplateRegex("messages/*"); got != "messages/[^/]+" {
		t.Fatalf("expected messages/[^/]+ got %s", got)
	}
}

func TestFiberTemplateUnary(t *testing.T) {
	got := (&serviceDesc{
		ServiceType: "Greeter",
		ServiceName: "helloworld.Greeter",
		Methods: []*methodDesc{
			{
				Name:         "SayHello",
				OriginalName: "SayHello",
				Request:      "HelloRequest",
				Reply:        "HelloReply",
				PathTemplate: "/helloworld/{name}",
				FiberPath:    "/helloworld/:name",
				FiberMethod:  "Get",
				Method:       "GET",
				HasVars:      true,
			},
		},
	}).execute()
	for _, want := range []string{
		`binding.Path(pattern, in, binding.WithQueryParams())`,
		`binding.Ensure(c)`,
		`c.Bind().URI(&in)`,
		`RegisterGreeterHTTPServer(r v3.Router`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated template missing %q in:\n%s", want, got)
		}
	}
}

func TestFiberTemplateStreamsAndHTTPBody(t *testing.T) {
	got := (&serviceDesc{
		ServiceType: "Greeter",
		ServiceName: "helloworld.Greeter",
		Methods: []*methodDesc{
			{
				Name:            "ListHello",
				OriginalName:    "ListHello",
				Request:         "ListHelloRequest",
				Reply:           "HelloReply",
				PathTemplate:    "/helloworld",
				FiberPath:       "/helloworld",
				FiberMethod:     "Get",
				Method:          "GET",
				ServerStreaming: true,
			},
			{
				Name:            "ChatHello",
				OriginalName:    "ChatHello",
				Request:         "HelloRequest",
				Reply:           "HelloReply",
				PathTemplate:    "/helloworld/chat",
				FiberPath:       "/helloworld/chat",
				FiberMethod:     "Get",
				Method:          "GET",
				ClientStreaming: true,
				ServerStreaming: true,
			},
			{
				Name:                 "UploadHello",
				OriginalName:         "UploadHello",
				Request:              "UploadHelloRequest",
				Reply:                "UploadHelloReply",
				PathTemplate:         "/helloworld/upload",
				FiberPath:            "/helloworld/upload",
				FiberMethod:          "Post",
				Method:               "POST",
				HasBody:              true,
				Body:                 ".Body",
				BodyField:            "body",
				BodyQueryName:        "body",
				BodyHTTPBody:         true,
				ResponseBody:         ".Body",
				ResponseBodyField:    "body",
				ResponseBodyHTTPBody: true,
			},
		},
	}).execute()
	for _, want := range []string{
		`ListHello(*ListHelloRequest, Greeter_ListHelloServer) error`,
		`binding.NewServerStream(_Greeter_ListHello0_SSE_Handler(srv))`,
		`ChatHello(Greeter_ChatHelloServer) error`,
		`binding.NewWebSocketHandler`,
		`stream, err := c.cc.ServerSentEvent(ctx, "GET", path, nil, opts...)`,
		`stream, err := c.cc.WebSocket(ctx, path, opts...)`,
		`binding.WithField("body")`,
		`binding.WithHTTPBody()`,
		`binding.BodyContentType(in.Body)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated template missing %q in:\n%s", want, got)
		}
	}
}
