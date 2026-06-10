package main

import (
	"flag"
	"log"

	"github.com/aqcool/protoc-gen-go-fiber/example/internal/conf"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/server"
	"github.com/aqcool/protoc-gen-go-fiber/example/internal/service"
)

func main() {
	confPath := flag.String("conf", "configs/config.yaml", "config file path")
	flag.Parse()

	bc, err := conf.Load(*confPath)
	if err != nil {
		log.Fatal(err)
	}

	app := server.NewHTTPServer(service.NewGreeterService())
	log.Printf("listening on %s", bc.Server.HTTP.Addr)
	log.Fatal(app.Listen(bc.Server.HTTP.Addr))
}
