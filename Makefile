VERSION:=1.3.0

all: build

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

lint: ## Run linters against the code
# The "revive" linter is the replacement for the classic "golint" utility.
	golangci-lint run -E revive

# The following go build command passes a build-time variable to the linker.
# https://belief-driven-design.com/build-time-variables-in-go-51439b26ef9/

build: fmt vet ## Build manager binary.
	go build -o bin/ -ldflags="-X github.com/jphx/kconfig/common.Version=${VERSION}" ./...

test: build ## Run unit tests.
	go test ./...

install: ## Build and install executable programs locally.
	go install -ldflags="-X github.com/jphx/kconfig/common.Version=${VERSION}" ./...

dist: ## Create distributable tar files for Linux and MacOS.
	@mkdir -p dist/work
	$(call platform-build,linux,amd64)
	$(call platform-build,darwin,amd64)
	$(call platform-build,darwin,arm64)
	@rm -r dist/work

define platform-build
	@rm -r dist/work/* 2>/dev/null || true
	GOOS=$1 GOARCH=$2 go build -o dist/work ./...
	cp -rpP setup/ dist/work/
	tar czf dist/kconfig-${VERSION}-$1-$2.tar.gz -C dist/work .
endef

.PHONY: all help fmt vet lint build test install dist
