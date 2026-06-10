# protoc-gen-go-fiber

基于 Protobuf [`google.api.http`](https://github.com/googleapis/googleapis/google/api/http.proto) 注解，为 [Fiber v3](https://docs.gofiber.io/) 生成 HTTP **服务端**与**客户端**代码的 `protoc` 插件。

设计参考 [Kratos protoc-gen-go-http](https://github.com/go-kratos/kratos/tree/main/cmd/protoc-gen-go-http)，采用 **「生成器厚、运行时薄」** 架构：路由与参数绑定逻辑写在生成代码里（直接调用 `c.Bind()`），运行时仅保留 protojson 编解码、路径构建和流式帧协议。

---

## 特性

- 生成 `*_fiber.pb.go`，与 `protoc-gen-go` 输出的 `*.pb.go` 并列
- 服务端：`RegisterXxxHTTPServer(r fiber.Router, srv)`，支持 `App` 与 `Group`
- 客户端：基于 `net/http` + `binding.Path`，与 Kratos 一致
- 一元 RPC、Server Streaming（SSE）、Client/Bidi Streaming（WebSocket）
- 支持 `google.api.HttpBody`、`response_body`、嵌套路径 `{book.id}`、路径模式 `{name=projects/*}`
- 零 bootstrap：handler 首行 `binding.Ensure(c)` 懒注册 CustomBinder

---

## 环境要求

| 依赖 | 版本 |
|---|---|
| Go | 1.25+ |
| protoc | 3.x / 4.x / 5.x |
| protoc-gen-go | 与项目 protobuf 版本匹配 |
| Fiber | v3（仅服务端运行时） |

---

## 安装

```bash
go install github.com/aqcool/protoc-gen-go-fiber@latest
```

本地开发（在本仓库根目录）：

```bash
make install
```

---

## 快速开始

### 1. 编写 proto

```protobuf
syntax = "proto3";

package helloworld.v1;

import "google/api/annotations.proto";

option go_package = "your/module/api/helloworld/v1;v1";

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {
    option (google.api.http) = {
      get: "/helloworld/{name}"
    };
  }
}

message HelloRequest { string name = 1; }
message HelloReply   { string message = 1; }
```

### 2. 生成代码

```bash
protoc \
  -I . \
  -I third_party \
  --go_out=. --go_opt=paths=source_relative \
  --go-fiber_out=. --go-fiber_opt=paths=source_relative \
  api/helloworld/v1/helloworld.proto
```

生成文件：

- `helloworld.pb.go` — 消息与服务定义（protoc-gen-go）
- `helloworld_fiber.pb.go` — HTTP 服务端/客户端（本插件）

### 3. 实现并启动服务端

```go
app := fiber.New()
v1.RegisterGreeterHTTPServer(app, &myService{})
app.Listen(":8000")
```

实现 `GreeterHTTPServer` 接口即可，**无需**调用 `binding.Register(app)`。

挂载到路由组：

```go
api := app.Group("/api/v1")
v1.RegisterGreeterHTTPServer(api, &myService{})
```

### 4. 调用客户端

```go
cc := binding.NewHTTPClient(http.DefaultClient, "http://127.0.0.1:8000")
client := v1.NewGreeterHTTPClient(cc)
reply, err := client.SayHello(ctx, &v1.HelloRequest{Name: "world"})
```

---

## 生成代码说明

### 服务端 handler（透明绑定，无 magic）

生成代码直接写出 Fiber 绑定顺序，便于阅读与调试：

```go
func _Greeter_SayHello0_HTTP_Handler(srv GreeterHTTPServer) v3.Handler {
    return func(c v3.Ctx) error {
        binding.Ensure(c)
        var in HelloRequest
        if err := c.Bind().Query(&in); err != nil { return err }
        if err := c.Bind().URI(&in); err != nil { return err }
        out, err := srv.SayHello(c, &in)
        if err != nil { return err }
        return binding.Write(c, v3.StatusOK, out)
    }
}
```

嵌套路径参数（如 `{book.id}`）会额外生成 `binding.SetURIParam` 回填。

### 生成的主要符号

| 符号 | 说明 |
|---|---|
| `XxxHTTPServer` | 服务端需实现的接口 |
| `RegisterXxxHTTPServer` | 注册路由到 `fiber.Router` |
| `OperationXxxYyy` | RPC 操作名常量，供中间件按操作过滤 |
| `XxxHTTPClient` | 客户端接口 |
| `NewXxxHTTPClient` | 构造客户端 |

---

## 运行时 binding 包

生成代码依赖 `github.com/aqcool/protoc-gen-go-fiber/binding`：

| API | 职责 |
|---|---|
| `Ensure(c)` | 懒注册 protojson CustomBinder（每个 App 一次） |
| `Write(c, status, msg, opts...)` | protojson 响应；支持 `WithField` / `WithHTTPBody` |
| `Path(template, msg, opts...)` | 客户端路径构建（等同 Kratos BuildPath） |
| `SetURIParam` | 嵌套 URI 字段赋值 |
| `BindHTTPBody` | `google.api.HttpBody` 请求体绑定 |
| `NewServerStream` / `NewWebSocketHandler` | 流式 handler 包装 |
| `NewHTTPClient` | 生成客户端的 HTTP 传输层 |

**刻意不提供** `BindRequest` 包装：绑定顺序由生成器按 method 描述直写，handler 内逻辑一目了然。

---

## 流式 RPC

| 类型 | 传输 | 帧格式 |
|---|---|---|
| Server Streaming | SSE（`middleware/sse`） | 每条 event `data` 为一帧 protojson |
| Client / Bidi Streaming | WebSocket（`contrib/v3/websocket`） | 每条 message 为一帧 protojson |

> 流式协议为自有 protojson 帧，**与 Kratos WebSocket `\x1e` 控制帧不互通**。

---

## 与 Kratos protoc-gen-go-http 的差异

| 项目 | Kratos | protoc-gen-go-fiber |
|---|---|---|
| 框架 | 自有 `transport/http` | Fiber v3 |
| 运行时体积 | ~15 个文件 | `binding/` 2 个文件 |
| 路由注册 | `RegisterXxx(s *http.Server, ...)` | `RegisterXxx(r fiber.Router, ...)` |
| 参数绑定 | `ctx.Bind` / `BindQuery` / `BindVars` | 生成代码直写 `c.Bind().URI/Query/Body` |
| 响应 | `ctx.Result` | `binding.Write` |
| 中间件 | 生成代码内嵌 Middleware 链 | 不生成；提供 `Operation*` 常量自选 |
| 客户端 | `net/http` | `net/http`（相同） |

---

## 项目结构

```
protoc-gen-go-fiber/
├── main.go                 # protoc 插件入口
├── fiber.go                # google.api.http 解析（移植自 Kratos）
├── template.go             # 模板数据结构
├── fiberTemplate.tpl       # 代码生成模板
├── binding/                # 用户项目 import 的薄运行时
│   ├── binding.go          # Ensure / Write / Path / HttpBody
│   └── stream.go           # SSE / WebSocket / HTTPClient
├── internal/pathutil/      # 编译期路径转换（仅生成器使用）
├── example/                # 独立 Go module 完整示例（Kratos layout）
├── testdata/               # 路径绑定测试 proto
├── third_party/            # google/api 等 proto
├── Makefile
└── buf.gen.yaml
```

---

## 开发与测试

```bash
# 插件单元测试 + testdata 生成测试
make tidy test

# 安装插件
make install

# 仅 regenerate testdata（example 在自身目录 generate）
make generate
```

完整集成示例（独立模块）：

```bash
cd example
make generate tidy test
make run-server   # terminal 1
make run-client   # terminal 2
```

详见 [example/README.md](example/README.md)。

---

## protoc 插件参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `omitempty` | `true` | 无 `google.api.http` 时不生成（设为 `false` 则为无注解 RPC 生成默认 POST） |
| `omitempty_prefix` | `""` | 上述默认 POST 路径前缀 |
| `paths=source_relative` | — | 与 protoc-gen-go 相同，输出到 proto 同目录 |

---

## buf 配置示例

根目录 `buf.gen.yaml`（testdata）：

```yaml
version: v2
plugins:
  - local: protoc-gen-go-fiber
    out: testdata/api
    opt: paths=source_relative
```

example 模块见 `example/buf.gen.yaml`。

---

## 路径转换规则（pathutil）

| Google API 路径 | Fiber 路由 | 绑定方式 |
|---|---|---|
| `/v1/books/{id}` | `/v1/books/:id` | `c.Bind().URI(&in)` |
| `/v1/books/{book.id}` | `/v1/books/:book_id` | URI + `SetURIParam(&in, "book.id", ...)` |
| `/v1/{name=projects/*}/m` | `/v1/:name<regex(...)>/m` | URI + 必要时 SetURIParam |

客户端 `binding.Path` 保留原始 `{field}` 模板（与 Kratos 一致）。

---

## License

见 [LICENSE](LICENSE) 文件。
