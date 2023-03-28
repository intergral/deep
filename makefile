# Version number
VERSION=$(shell ./tools/image-tag | cut -d, -f 1)

GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

GOPATH := $(shell go env GOPATH)
GORELEASER := $(GOPATH)/bin/goreleaser

# Build Images
DOCKER_PROTOBUF_IMAGE ?= otel/build-protobuf:0.14.0
LOKI_BUILD_IMAGE ?= grafana/loki-build-image:0.21.0
DOCS_IMAGE ?= grafana/docs-base:latest

# More exclusions can be added similar with: -not -path './testbed/*'
ALL_SRC := $(shell find . -name '*.go' \
								-not -path './vendor*/*' \
								-not -path './integration/*' \
								-not -path './cmd/deep-serverless/*' \
                                -type f | sort)

# All source code and documents. Used in spell check.
ALL_DOC := $(shell find . \( -name "*.md" -o -name "*.yaml" \) \
                                -type f | sort)

# ALL_PKGS is used with 'go cover'
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

GO_OPT= -mod vendor -ldflags "-X main.Branch=$(GIT_BRANCH) -X main.Revision=$(GIT_REVISION) -X main.Version=$(VERSION)"
ifeq ($(BUILD_DEBUG), 1)
	GO_OPT+= -gcflags="all=-N -l"
endif

GOTEST_OPT?= -race -timeout 30m -count=1
GOTEST_OPT_WITH_COVERAGE = $(GOTEST_OPT) -cover
GOTEST=go test
LINT=golangci-lint

UNAME := $(shell uname -s)
ifeq ($(UNAME), Darwin)
    SED_OPTS := ''
endif

FILES_TO_FMT=$(shell find . -type d \( -path ./vendor -o -path ./opentelemetry-proto -o -path ./vendor-fix \) -prune -o -name '*.go' -not -name "*.pb.go" -not -name '*.y.go' -print)
FILES_TO_JSONNETFMT=$(shell find ./operations/jsonnet ./operations/deep-mixin -type f \( -name '*.libsonnet' -o -name '*.jsonnet' \) -not -path "*/vendor/*" -print)

### Build

.PHONY: deep
deep:
	GO111MODULE=on CGO_ENABLED=0 go build $(GO_OPT) -o ./bin/$(GOOS)/deep-$(GOARCH) $(BUILD_INFO) ./cmd/deep
