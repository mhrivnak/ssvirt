.PHONY: all build test integration-test container-build generate deploy clean lint fmt vet

# Default target – so `make` without args does something useful
all: build

build:
	go build -o bin/api-server ./cmd/api-server
	go build -o bin/controller ./cmd/controller

test:
	go test ./...

integration-test:
	go test ./test/integration/...

container-build:
	podman build -t ssvirt:latest .

generate:
	go generate ./...
	controller-gen crd paths=./pkg/api/... output:crd:dir=./manifests/crd

deploy:
	kubectl apply -f manifests/

clean:
	rm -rf bin/

lint:
	golangci-lint run

fmt:
	go fmt ./...

vet:
	go vet ./...