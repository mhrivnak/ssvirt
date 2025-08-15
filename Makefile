.PHONY: all build test integration-test container-build generate deploy clean lint fmt vet

# Default target – so `make` without args does something useful
all: build

build:
	go build -o bin/api-server ./cmd/api-server
	go build -o bin/user-admin ./cmd/user-admin
	go build -o bin/vm-controller ./cmd/vm-controller

test:
	go test $(shell go list ./... | grep -v '.disabled')

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