include ../../common.mk

.PHONY: config
# generate config proto
config: check-buf-tool
	cd ./internal/conf && buf generate

.PHONY: api
# generate api proto
api: check-buf-tool
	cd ./api && buf generate

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags\
	  "-X github.com/chainloop-dev/chainloop/app/artifact-cas/internal/server.Version=$(VERSION)\
	  -X main.Version=$(VERSION)"\
	   -o ./bin/artifact-cas ./cmd/...

.PHONY: run
# run
run:
	go run ./cmd/... --conf ./configs

.PHONY: test
# test
test:
	go test ./...

.PHONY: lint
# lint
lint: check-golangci-lint-tool check-buf-tool
	golangci-lint run
	buf lint api
	buf lint internal/conf

.PHONY: generate
# generate
generate: check-wire-tool api config
	go generate ./...

.PHONY: all
# generate all
all:
	make config;
	make generate;
