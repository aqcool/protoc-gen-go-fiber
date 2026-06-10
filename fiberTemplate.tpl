{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

{{- range .MethodSets}}
const Operation{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

type {{.ServiceType}}HTTPServer interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{- if .ClientStreaming}}
	{{.Name}}({{$svrType}}_{{.Name}}Server) error
	{{- else if .ServerStreaming}}
	{{.Name}}(*{{.Request}}, {{$svrType}}_{{.Name}}Server) error
	{{- else}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
	{{- end}}
{{- end}}
}

func Register{{.ServiceType}}HTTPServer(r v3.Router, srv {{.ServiceType}}HTTPServer) {
	{{- range .Methods}}
	{{- if .ClientStreaming}}
	r.Get("{{.FiberPath}}", binding.NewWebSocketHandler(func(stream *binding.WebSocketStream) error {
		return _{{$svrType}}_{{.Name}}{{.Num}}_WS_Handler(srv, stream)
	}))
	{{- else if .ServerStreaming}}
	r.{{if eq .FiberMethod "Add"}}Add([]string{"{{.Method}}"}, "{{.FiberPath}}", {{else}}{{.FiberMethod}}("{{.FiberPath}}", {{end}}binding.NewServerStream(_{{$svrType}}_{{.Name}}{{.Num}}_SSE_Handler(srv)))
	{{- else if eq .FiberMethod "Add"}}
	r.Add([]string{"{{.Method}}"}, "{{.FiberPath}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- else}}
	r.{{.FiberMethod}}("{{.FiberPath}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end}}
	{{- end}}
}

{{range .MethodSets}}
{{- if or .ClientStreaming .ServerStreaming}}
type {{$svrType}}_{{.Name}}Server interface {
{{- if .ServerStreaming}}
	Send(*{{.Reply}}) error
{{- end}}
{{- if .ClientStreaming}}
	Recv() (*{{.Request}}, error)
{{- end}}
{{- if and .ClientStreaming (not .ServerStreaming)}}
	SendAndClose(*{{.Reply}}) error
{{- end}}
}

type {{$svrType}}_{{.Name}}HTTPServer struct {
	{{- if .ServerStreaming}}
	stream *binding.ServerStream
	{{- end}}
	{{- if .ClientStreaming}}
	ws *binding.WebSocketStream
	{{- end}}
}

{{- if .ServerStreaming}}
func (x *{{$svrType}}_{{.Name}}HTTPServer) Send(m *{{.Reply}}) error {
	{{- if .ClientStreaming}}
	return x.ws.Send(m)
	{{- else}}
	return x.stream.Send(m)
	{{- end}}
}
{{- end}}

{{- if .ClientStreaming}}
func (x *{{$svrType}}_{{.Name}}HTTPServer) Recv() (*{{.Request}}, error) {
	m := new({{.Request}})
	if err := x.ws.Recv(m); err != nil {
		return nil, err
	}
	return m, nil
}
{{- end}}

{{- if and .ClientStreaming (not .ServerStreaming)}}
func (x *{{$svrType}}_{{.Name}}HTTPServer) SendAndClose(m *{{.Reply}}) error {
	if err := x.ws.Send(m); err != nil {
		return err
	}
	return x.ws.Close()
}
{{- end}}
{{- end}}
{{end}}

{{range .Methods}}
{{- if .ClientStreaming}}
func _{{$svrType}}_{{.Name}}{{.Num}}_WS_Handler(srv {{$svrType}}HTTPServer, ws *binding.WebSocketStream) error {
	return srv.{{.Name}}(&{{$svrType}}_{{.Name}}HTTPServer{ws: ws})
}
{{- else if .ServerStreaming}}
func _{{$svrType}}_{{.Name}}{{.Num}}_SSE_Handler(srv {{$svrType}}HTTPServer) func(c v3.Ctx, stream *binding.ServerStream) error {
	return func(c v3.Ctx, stream *binding.ServerStream) error {
		binding.Ensure(c)
		var in {{.Request}}
		{{- if .HasBody}}
		{{- if .BodyHTTPBody}}
		if err := binding.BindHTTPBody(c, &in, "{{.BodyField}}"); err != nil {
			return err
		}
		{{- else}}
		if err := c.Bind().Body(&in{{.Body}}); err != nil {
			return err
		}
		{{- end}}
		{{- end}}
		{{- if not .HasBody}}
		if err := c.Bind().Query(&in); err != nil {
			return err
		}
		{{- else if ne .BodyField "*"}}
		if err := c.Bind().Query(&in); err != nil {
			return err
		}
		{{- end}}
		{{- if or .HasVars (len .URIParams)}}
		if err := c.Bind().URI(&in); err != nil {
			return err
		}
		{{- end}}
		{{- range .URIParams}}
		{{- if isNestedURI .FieldPath}}
		if err := binding.SetURIParam(&in, "{{.FieldPath}}", c.Params("{{.ParamName}}")); err != nil {
			return err
		}
		{{- end}}
		{{- end}}
		return srv.{{.Name}}(&in, &{{$svrType}}_{{.Name}}HTTPServer{stream: stream})
	}
}
{{- else}}
func _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv {{$svrType}}HTTPServer) v3.Handler {
	return func(c v3.Ctx) error {
		binding.Ensure(c)
		var in {{.Request}}
		{{- if .HasBody}}
		{{- if .BodyHTTPBody}}
		if err := binding.BindHTTPBody(c, &in, "{{.BodyField}}"); err != nil {
			return err
		}
		{{- else}}
		if err := c.Bind().Body(&in{{.Body}}); err != nil {
			return err
		}
		{{- end}}
		{{- end}}
		{{- if not .HasBody}}
		if err := c.Bind().Query(&in); err != nil {
			return err
		}
		{{- else if ne .BodyField "*"}}
		if err := c.Bind().Query(&in); err != nil {
			return err
		}
		{{- end}}
		{{- if or .HasVars (len .URIParams)}}
		if err := c.Bind().URI(&in); err != nil {
			return err
		}
		{{- end}}
		{{- range .URIParams}}
		{{- if isNestedURI .FieldPath}}
		if err := binding.SetURIParam(&in, "{{.FieldPath}}", c.Params("{{.ParamName}}")); err != nil {
			return err
		}
		{{- end}}
		{{- end}}
		out, err := srv.{{.Name}}(c, &in)
		if err != nil {
			return err
		}
		{{- if .ResponseBodyHTTPBody}}
		return binding.Write(c, v3.StatusOK, out, binding.WithField("{{.ResponseBodyField}}"), binding.WithHTTPBody())
		{{- else if .ResponseBody}}
		return binding.Write(c, v3.StatusOK, out, binding.WithField("{{.ResponseBodyField}}"))
		{{- else if .ReplyHTTPBody}}
		return binding.Write(c, v3.StatusOK, out, binding.WithHTTPBody())
		{{- else}}
		return binding.Write(c, v3.StatusOK, out)
		{{- end}}
	}
}
{{- end}}
{{end}}

type {{.ServiceType}}HTTPClient interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{- if .ClientStreaming}}
	{{.Name}}(ctx context.Context, opts ...binding.CallOption) ({{$svrType}}_{{.Name}}Client, error)
	{{- else if .ServerStreaming}}
	{{.Name}}(ctx context.Context, req *{{.Request}}, opts ...binding.CallOption) ({{$svrType}}_{{.Name}}Client, error)
	{{- else}}
	{{.Name}}(ctx context.Context, req *{{.Request}}, opts ...binding.CallOption) (*{{.Reply}}, error)
	{{- end}}
{{- end}}
}

type {{.ServiceType}}HTTPClientImpl struct {
	cc *binding.HTTPClient
}

func New{{.ServiceType}}HTTPClient(cc *binding.HTTPClient) {{.ServiceType}}HTTPClient {
	return &{{.ServiceType}}HTTPClientImpl{cc: cc}
}

{{range .MethodSets}}
{{- if or .ClientStreaming .ServerStreaming}}
type {{$svrType}}_{{.Name}}Client interface {
{{- if .ServerStreaming}}
	Recv() (*{{.Reply}}, error)
{{- end}}
{{- if .ClientStreaming}}
	Send(*{{.Request}}) error
	CloseSend() error
{{- end}}
{{- if and .ClientStreaming (not .ServerStreaming)}}
	CloseAndRecv() (*{{.Reply}}, error)
{{- end}}
	Close() error
}

type {{$svrType}}_{{.Name}}HTTPClient struct {
	{{- if .ClientStreaming}}
	stream *binding.WSClientStream
	{{- else}}
	stream *binding.ClientStream
	{{- end}}
}
{{- end}}
{{end}}

{{range .MethodSets}}
{{- if .ClientStreaming}}
func (c *{{$svrType}}HTTPClientImpl) {{.Name}}(ctx context.Context, opts ...binding.CallOption) ({{$svrType}}_{{.Name}}Client, error) {
	pattern := "{{.PathTemplate}}"
	path := binding.Path(pattern, nil, binding.WithQueryParams())
	stream, err := c.cc.WebSocket(ctx, path, opts...)
	if err != nil {
		return nil, err
	}
	return &{{$svrType}}_{{.Name}}HTTPClient{stream: stream}, nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) Send(m *{{.Request}}) error {
	return x.stream.Send(m)
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) CloseSend() error {
	return nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) Recv() (*{{.Reply}}, error) {
	m := new({{.Reply}})
	if err := x.stream.Recv(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) CloseAndRecv() (*{{.Reply}}, error) {
	m := new({{.Reply}})
	if err := x.stream.Recv(m); err != nil {
		return nil, err
	}
	_ = x.stream.Close()
	return m, nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) Close() error {
	if x.stream == nil {
		return nil
	}
	return x.stream.Close()
}
{{- else if .ServerStreaming}}
func (c *{{$svrType}}HTTPClientImpl) {{.Name}}(ctx context.Context, in *{{.Request}}, opts ...binding.CallOption) ({{$svrType}}_{{.Name}}Client, error) {
	pattern := "{{.PathTemplate}}"
	{{- if .HasBody}}
		{{- if or (eq .BodyField "*") (eq .BodyField "")}}
	path := binding.Path(pattern, in)
		{{- else}}
	path := binding.Path(pattern, in, binding.WithQueryParams(), binding.WithOmitFields("{{.BodyQueryName}}"))
		{{- end}}
	stream, err := c.cc.ServerSentEvent(ctx, "{{.Method}}", path, in{{.Body}}, opts...)
	{{- else}}
	path := binding.Path(pattern, in, binding.WithQueryParams())
	stream, err := c.cc.ServerSentEvent(ctx, "{{.Method}}", path, nil, opts...)
	{{- end}}
	if err != nil {
		return nil, err
	}
	return &{{$svrType}}_{{.Name}}HTTPClient{stream: stream}, nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) Recv() (*{{.Reply}}, error) {
	m := new({{.Reply}})
	if err := x.stream.Recv(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *{{$svrType}}_{{.Name}}HTTPClient) Close() error {
	if x.stream == nil {
		return nil
	}
	return x.stream.Close()
}
{{- else}}
func (c *{{$svrType}}HTTPClientImpl) {{.Name}}(ctx context.Context, in *{{.Request}}, opts ...binding.CallOption) (*{{.Reply}}, error) {
	var out {{.Reply}}
	pattern := "{{.PathTemplate}}"
	{{- if .HasBody}}
		{{- if or (eq .BodyField "*") (eq .BodyField "")}}
	path := binding.Path(pattern, in)
		{{- else}}
	path := binding.Path(pattern, in, binding.WithQueryParams(), binding.WithOmitFields("{{.BodyQueryName}}"))
		{{- end}}
	opts = append([]binding.CallOption{
		binding.Header("Accept", "application/json"),
		{{- if .BodyHTTPBody}}
		binding.Header("Content-Type", binding.BodyContentType(in{{.Body}})),
		{{- else}}
		binding.Header("Content-Type", "application/json"),
		{{- end}}
	}, opts...)
	{{- if .ResponseBodyHTTPBody}}
	if out{{.ResponseBody}} == nil {
		out{{.ResponseBody}} = &httpbody.HttpBody{}
	}
	{{- end}}
	{{- if .ResponseBodyHTTPBody}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, in{{.Body}}, out{{.ResponseBody}}, opts...)
	{{- else if .ResponseBody}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, in{{.Body}}, &out{{.ResponseBody}}, opts...)
	{{- else}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, in{{.Body}}, &out, opts...)
	{{- end}}
	{{- else}}
	path := binding.Path(pattern, in, binding.WithQueryParams())
	opts = append([]binding.CallOption{binding.Header("Accept", "application/json")}, opts...)
	{{- if .ResponseBodyHTTPBody}}
	if out{{.ResponseBody}} == nil {
		out{{.ResponseBody}} = &httpbody.HttpBody{}
	}
	{{- end}}
	{{- if .ResponseBodyHTTPBody}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, nil, out{{.ResponseBody}}, opts...)
	{{- else if .ResponseBody}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, nil, &out{{.ResponseBody}}, opts...)
	{{- else}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, nil, &out, opts...)
	{{- end}}
	{{- end}}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
{{- end}}
{{end}}
