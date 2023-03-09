VERSION=$(shell git describe --tags --always)

.PHONY: config
# generate config proto
config:
	cd ./internal/conf && buf generate

.PHONY: api
# generate api proto
api:
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
lint:
	golangci-lint run
	buf lint api
	buf lint internal/conf

.PHONY: generate
# generate
generate:
	go generate ./...

.PHONY: all
# generate all
all:
	make config;
	make generate;

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
