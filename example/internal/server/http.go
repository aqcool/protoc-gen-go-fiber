// Package server 组装 Fiber App 并注册生成的 HTTP 路由。
package server

import (
	"github.com/gofiber/fiber/v3"

	v1 "github.com/aqcool/protoc-gen-go-fiber/example/api/helloworld/v1"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/service"
)

// NewHTTPServer creates a Fiber app with generated Greeter routes registered.
func NewHTTPServer(greeter *service.GreeterService) *fiber.App {
	app := fiber.New()
	v1.RegisterGreeterHTTPServer(app, greeter)
	return app
}
