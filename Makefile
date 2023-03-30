# Version number
VERSION=$(shell ./tools/image-tag | cut -d, -f 1)

GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

FILES_TO_FMT=$(shell find . -type d \( -path ./vendor \) -prune -o -name '*.go' -not -name "*.pb.go" -not -name '*.y.go' -print)

GO_OPT= -mod vendor -ldflags "-X main.Branch=$(GIT_BRANCH) -X main.Revision=$(GIT_REVISION) -X main.Version=$(VERSION)"
ifeq ($(BUILD_DEBUG), 1)
	GO_OPT+= -gcflags="all=-N -l"
endif

### Build

.PHONY: deep
deep:
	GO111MODULE=on CGO_ENABLED=0 go build $(GO_OPT) -o ./bin/$(GOOS)/deep-$(GOARCH) $(BUILD_INFO) ./cmd/deep

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component exe
	docker build -t deep/$(COMPONENT) --build-arg=TARGETARCH=$(GOARCH) -f ./cmd/$(COMPONENT)/Dockerfile .
	docker tag deep/$(COMPONENT) $(COMPONENT)

.PHONY: docker-component-debug
docker-component-debug: check-component exe-debug
	docker build -t deep/$(COMPONENT)-debug --build-arg=TARGETARCH=$(GOARCH) -f ./cmd/$(COMPONENT)/Dockerfile_debug .
	docker tag deep/$(COMPONENT)-debug $(COMPONENT)-debug

.PHONY: docker-deep
docker-deep:
	COMPONENT=deep $(MAKE) docker-component

docker-deep-debug:
	COMPONENT=deep $(MAKE) docker-component-debug

.PHONY: check-component
check-component:
ifndef COMPONENT
	$(error COMPONENT variable was not defined)
endif

.PHONY: exe
exe:
	GOOS=linux $(MAKE) $(COMPONENT)

.PHONY: exe-debug
exe-debug:
	BUILD_DEBUG=1 GOOS=linux $(MAKE) $(COMPONENT)


.PHONY: fmt check-fmt
fmt:
	echo $(FILES_TO_FMT)
	@gofmt -s -w $(FILES_TO_FMT)
	@goimports -w $(FILES_TO_FMT)

check-fmt: fmt
	@git diff --exit-code -- $(FILES_TO_FMT)