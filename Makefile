.PHONY: build install test generate tidy example-test

build:
	go build -o bin/protoc-gen-go-fiber .

install:
	go install .

test:
	go test ./...
	go test ./testdata/api/path/v1/
	$(MAKE) -C example test

example-test:
	$(MAKE) -C example test

tidy:
	go mod tidy

PROTO_INCLUDE ?= $(shell find /opt/homebrew/include /usr/local/include -path '*/google/protobuf/any.proto' 2>/dev/null | head -1 | sed 's|/google/protobuf/any.proto||')

generate: install
	protoc \
		-I testdata/api \
		-I third_party \
		-I $(PROTO_INCLUDE) \
		--go_out=testdata/api --go_opt=paths=source_relative \
		--go-fiber_out=testdata/api --go-fiber_opt=paths=source_relative \
		path/v1/path.proto
