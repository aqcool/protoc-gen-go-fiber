.PHONY: generate tidy test run-server run-client

PROTO_INCLUDE ?= $(shell find /opt/homebrew/include /usr/local/include -path '*/google/protobuf/any.proto' 2>/dev/null | head -1 | sed 's|/google/protobuf/any.proto||')

generate:
	@command -v protoc-gen-go-fiber >/dev/null || (echo "install plugin: make -C .. install" && exit 1)
	protoc \
		-I api \
		-I third_party \
		-I $(PROTO_INCLUDE) \
		--go_out=api --go_opt=paths=source_relative \
		--go-fiber_out=api --go-fiber_opt=paths=source_relative \
		api/helloworld/v1/helloworld.proto

tidy:
	go mod tidy

test:
	go test ./...

run-server:
	go run ./cmd/server -conf configs/config.yaml

run-client:
	go run ./cmd/client -conf configs/config.yaml
