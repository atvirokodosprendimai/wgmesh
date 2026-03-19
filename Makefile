.PHONY: build clean install test fmt lint deps proto

build:
	go build -o wgmesh

install:
	go install

clean:
	rm -f wgmesh mesh-state.json

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy

proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/rpc/proto/wgmesh.proto
