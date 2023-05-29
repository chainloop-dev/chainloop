VERSION=$(shell git describe --tags --always)

API_PROTO_FILES=$(shell find api -name *.proto)

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	go install github.com/envoyproxy/protoc-gen-validate@v1.0.1
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/vektra/mockery/v2@v2.20.0
	go install entgo.io/ent/cmd/ent@v0.11.4
	go install github.com/bufbuild/buf/cmd/buf@v1.10.0

.PHONY: api
# generate api proto
api:
	make -C ./app/controlplane api
	make -C ./app/cli api
	make -C ./app/artifact-cas api

.PHONY: config
# generate config proto
config:
	cd ./internal/credentials/api && buf generate
	make -C ./app/controlplane config
	make -C ./app/artifact-cas config

.PHONY: generate

# generate
generate: config api
	go generate ./...

.PHONY: all
# generate all
all:
	make generate;
	make api;

.PHONY: lint
# run linter
lint:
	golangci-lint run
	buf lint
	make -C ./app/controlplane lint
	make -C ./app/cli lint
	make -C ./app/artifact-cas lint

.PHONY: test
# All tests, both unit and integration
test: 
	go test ./... 

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
