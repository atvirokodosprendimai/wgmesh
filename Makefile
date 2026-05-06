.PHONY: build clean install test test-relay pulse-smoke

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

pulse-smoke:
	WINDOW="$${WINDOW:-24h}" POLAR_TOKEN="$${POLAR_TOKEN:-}" COROOT_API_TOKEN="$${COROOT_API_TOKEN:-}" \
	  GITHUB_TOKEN="$${GITHUB_TOKEN:-}" GH_REPO="$${GH_REPO:-atvirokodosprendimai/wgmesh}" bash scripts/pulse.sh

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy
