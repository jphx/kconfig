all: build

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golint against code.
	golint ./...

build: fmt vet ## Build manager binary.
	go build -o bin/ ./...

install:
	go install ./...

.PHONY: all fmt vet lint build install
