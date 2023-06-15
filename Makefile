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

# More exclusions can be added similar with: -not -path './testbed/*'
ALL_SRC := $(shell find . -name '*.go' \
								-not -path './tools*/*' \
								-not -path './vendor*/*' \
                                -type f | sort)

# ALL_PKGS is used with 'go cover'
ALL_PKGS := $(shell go list $(sort $(dir $(ALL_SRC))))

## Tests
GOTEST_OPT?= -race -timeout 20m -count=1 -v
GOTEST_OPT_WITH_COVERAGE = $(GOTEST_OPT) -cover
GOTEST=gotestsum --format=testname --

.PHONY: test
test:
	$(GOTEST) $(GOTEST_OPT) $(ALL_PKGS)

.PHONY: test-with-cover
test-with-cover:
	$(GOTEST) $(GOTEST_OPT_WITH_COVERAGE) $(ALL_PKGS)

### Build

.PHONY: deep
deep:
	GO111MODULE=on CGO_ENABLED=0 go build $(GO_OPT) -o ./bin/$(GOOS)/deep-$(GOARCH) $(BUILD_INFO) ./cmd/deep

.PHONY: deep-cli
deep-cli:
	GO111MODULE=on CGO_ENABLED=0 go build $(GO_OPT) -o ./bin/$(GOOS)/deep-cli-$(GOARCH) $(BUILD_INFO) ./cmd/deep-cli

.PHONY: docker-component # Not intended to be used directly
docker-component: check-component exe
	docker build -t intergral/$(COMPONENT) --build-arg=TARGETARCH=$(GOARCH) -f ./cmd/$(COMPONENT)/Dockerfile .

.PHONY: docker-component-debug
docker-component-debug: check-component exe-debug
	docker build -t intergral/$(COMPONENT)-debug --build-arg=TARGETARCH=$(GOARCH) -f ./cmd/$(COMPONENT)/Dockerfile_debug .

.PHONY: docker-deep-ci
docker-deep-ci: update-mod docker-deep

.PHONY: docker-deep
docker-deep:
	COMPONENT=deep $(MAKE) docker-component

.PHONY: docker-deep-cli
docker-deep-cli:
	COMPONENT=deep-cli $(MAKE) docker-component

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


# #########
# Gen Proto
# #########
UNAME := $(shell uname -s)
ifeq ($(UNAME), Darwin)
    SED_OPTS := ''
endif
DOCKER_PROTOBUF_IMAGE ?= otel/build-protobuf:0.14.0
PROTOC = docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD} ${DOCKER_PROTOBUF_IMAGE}
PROTO_INTERMEDIATE_DIR = pkg/.patched-proto

PROTOC_ARGS=--go_out=${PROTO_INTERMEDIATE_DIR}/out
GRPC_PROTOC_ARGS=--go-grpc_out=${PROTO_INTERMEDIATE_DIR}/out

.PHONY: gen-proto
gen-proto:
	@echo --
	@echo -- Deleting existing
	@echo --
	rm -rf deep-proto
	rm -rf $(PROTO_INTERMEDIATE_DIR)
	find pkg/deeppb -name *.pb.go | xargs -L 1 -I rm
	# Here we avoid removing our deep.proto and our frontend.proto due to reliance on the gogoproto bits.
	find pkg/deeppb -name *.proto | grep -v deep.proto | grep -v frontend.proto | xargs -L 1 -I rm

	@echo --
	@echo -- Copying to $(PROTO_INTERMEDIATE_DIR)
	@echo --
	git submodule update --init
	cd deep-proto && git pull origin master
	mkdir -p $(PROTO_INTERMEDIATE_DIR)
	mkdir -p $(PROTO_INTERMEDIATE_DIR)/out
	mkdir -p $(PROTO_INTERMEDIATE_DIR)/stats
	mkdir -p $(PROTO_INTERMEDIATE_DIR)/frontend
	cp -R deep-proto/deepproto/proto/* $(PROTO_INTERMEDIATE_DIR)
	cp pkg/deeppb/deep.proto $(PROTO_INTERMEDIATE_DIR)

	@echo --
	@echo -- Editing proto
	@echo --

	@# Update package and types from opentelemetry.proto.* -> deeppb.*
	@# giving final types like "deeppb.common.v1.InstrumentationLibrary" which
	@# will not conflict with other usages of opentelemetry proto in downstream apps.
	find $(PROTO_INTERMEDIATE_DIR) -name "*.proto" | xargs -L 1 sed -i $(SED_OPTS) 's+ deepproto.proto+ deeppb+g'

	@# Update go_package
	find $(PROTO_INTERMEDIATE_DIR) -name "*.proto" | xargs -L 1 sed -i $(SED_OPTS) 's+github.com/intergral/go-deep-proto+github.com/intergral/deep/pkg/deeppb+g'

	@# Update import paths
	find $(PROTO_INTERMEDIATE_DIR) -name "*.proto" | xargs -L 1 sed -i $(SED_OPTS) 's+import "deepproto/proto/+import "+g'

	@echo --
	@echo -- Gen proto --
	@echo --
	# Generate standard go proto from all files
	$(foreach file, $(shell find ${PROTO_INTERMEDIATE_DIR} -name '*.proto'), ${PROTOC} --proto_path ${PROTO_INTERMEDIATE_DIR} ${PROTOC_ARGS} $(file);)

	# Now generate grpc proto from specific files
	${PROTOC} --proto_path ${PROTO_INTERMEDIATE_DIR} ${PROTOC_ARGS} ${GRPC_PROTOC_ARGS} ${PROTO_INTERMEDIATE_DIR}/poll/v1/poll.proto
	${PROTOC} --proto_path ${PROTO_INTERMEDIATE_DIR} ${PROTOC_ARGS} ${GRPC_PROTOC_ARGS} ${PROTO_INTERMEDIATE_DIR}/tracepoint/v1/tracepoint.proto
	${PROTOC} --proto_path ${PROTO_INTERMEDIATE_DIR} ${PROTOC_ARGS} ${GRPC_PROTOC_ARGS} ${PROTO_INTERMEDIATE_DIR}/deep.proto

	# Generate stats proto
	${PROTOC} --proto_path modules/querier/ --go_out=${PROTO_INTERMEDIATE_DIR}/stats --go-grpc_out=${PROTO_INTERMEDIATE_DIR}/stats modules/querier/stats/stats.proto
	# Generate frontend proto
	${PROTOC} -Imodules --proto_path modules/frontend/ --go_out=${PROTO_INTERMEDIATE_DIR}/frontend --go-grpc_out=${PROTO_INTERMEDIATE_DIR}/frontend modules/frontend/v1/frontendv1pb/frontend.proto

	@echo --
	@echo -- Move output --
	@echo --
	cp -r $(PROTO_INTERMEDIATE_DIR)/out/github.com/intergral/deep/pkg/deeppb/* pkg/deeppb
	cp -r $(PROTO_INTERMEDIATE_DIR)/stats/github.com/intergral/deep/modules/querier/stats/* modules/querier/stats
	cp -r $(PROTO_INTERMEDIATE_DIR)/frontend/github.com/intergral/deep/modules/frontend/v1/frontendv1pb/* modules/frontend/v1/frontendv1pb

	@echo --
	@echo -- Clean up --
	@echo --
	rm -rf $(PROTO_INTERMEDIATE_DIR)

# ##############
# Gen deepql
# ##############
GO_YACC_IMAGE ?= grafana/loki-build-image:0.21.0

.PHONY: gen-deepql
gen-deepql:
	docker run --rm -v${PWD}:/src/loki ${GO_YACC_IMAGE} gen-deepql-local

.PHONY: gen-deepql-local
gen-deepql-local:
	goyacc -o pkg/deepql/expr.y.go pkg/deepql/expr.y

### Check vendored and generated files are up to date
.PHONY: vendor-check
vendor-check: gen-proto update-mod gen-deepql
	git diff --exit-code -- **/go.sum **/go.mod vendor/ pkg/deeppb/ pkg/deepql/ modules/querier/stats modules/frontend/v1/frontendv1pb

### Tidy dependencies for tempo and tempo-serverless modules
.PHONY: update-mod
update-mod:
	go mod vendor
	go mod tidy -e


.PHONY: docs
docs:
	mkdocs build -f ./docs/mkdocs.yml

.PHONY: docs-serve
docs-serve:
	mkdocs serve -f ./docs/mkdocs.yml
