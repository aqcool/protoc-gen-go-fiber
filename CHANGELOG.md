# Changelog

## v0.1.0 — 2026-06-10

### Added

- `protoc-gen-go-fiber` plugin: generate `*_fiber.pb.go` from `google.api.http` annotations
- `binding` runtime: `Ensure`, `Write`, `Path`, HttpBody, SSE/WebSocket streaming
- Unary, server streaming (SSE), client/bidi streaming (WebSocket), `google.api.HttpBody`
- `example/` standalone module (Kratos layout) with full RPC integration tests
- `internal/pathutil` for Google API path → Fiber route conversion
