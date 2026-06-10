# Helloworld 示例

独立 Go module，目录布局参考 [Kratos layout](https://go-kratos.dev/docs/intro/layout/)，用于端到端验证 `protoc-gen-go-fiber` 生成的全部 RPC 类型。

## 目录结构

```
example/
├── api/helloworld/v1/     # proto 与生成代码 (*.pb.go / *_fiber.pb.go)
├── cmd/
│   ├── server/            # HTTP 服务入口
│   └── client/            # HTTP 客户端演示
├── configs/config.yaml    # 监听地址与客户端 endpoint
├── internal/
│   ├── conf/              # YAML 配置加载
│   ├── server/            # Fiber App 组装与路由注册
│   └── service/           # GreeterHTTPServer 业务实现
├── third_party/           # google/api proto
├── go.mod                 # 独立模块（replace 指向父仓库）
├── Makefile
└── buf.gen.yaml
```

## 快速开始

在仓库根目录安装插件后：

```bash
make install
cd example
make generate tidy test
make run-server           # 终端 1
make run-client           # 终端 2
```

或在根目录：

```bash
make -C example run-server
make -C example run-client
```

## 测试

```bash
make test
```

`internal/server/http_test.go` 覆盖全部 5 个 RPC：

| RPC | Server (`app.Test` / TCP) | Client (生成代码) |
|---|---|---|
| SayHello | ✅ | ✅ |
| CreateHello | ✅ | ✅ |
| ListHello | ✅（3 帧 SSE） | ✅（3 帧 SSE） |
| ChatHello | ✅（WebSocket / TCP） | ✅ |
| UploadHello | ✅ | ✅ |


## 本地依赖

`go.mod` 中使用：

```go
replace github.com/aqcool/protoc-gen-go-fiber => ../
```

发布到生产环境时，将 `replace` 改为正式版本号即可。
