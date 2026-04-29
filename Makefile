.PHONY: build clean install test test-relay

build:
	go build -o wgmesh

install:
	go install

clean:
	rm -f wgmesh mesh-state.json

test:
	go test ./...

test-relay:
	MESH_SECRET="${MESH_SECRET:-wgmesh://v1/cmVsYXktaW50ZWdyYXRpb24tdGVzdA}" \
	  bash testlab/nat-relay/run-test.sh

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy
