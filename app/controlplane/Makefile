VERSION=$(shell git describe --tags --always)

.PHONY: config
# generate config proto bindings
config:
	cd ./internal/conf && buf generate

.PHONY: api
# generate api proto bindings
api:
	cd ./api && buf generate

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags\
	  "-X github.com/chainloop-dev/chainloop/app/controlplane/internal/server.Version=$(VERSION)\
	  -X main.Version=$(VERSION)"\
	  -o ./bin/control-plane ./cmd/...

.PHONY: run
# run
run:
	go run ./cmd/... --conf ./configs

.PHONY: test
# All tests, both unit and integration
test: 
	go test $$(go list ./... | grep -v /internal/data/ent)

.PHONY: test-unit
# unit testing, excluding integration tests
test-unit:
	SKIP_INTEGRATION=true make test

.PHONY: lint
# lint
lint:
	buf lint api
	buf lint internal/conf
	golangci-lint run

.PHONY: generate
# generate proto bindings, wire injectors, and ent models
generate: api config
	go generate ./...

.PHONY: all
# generate all
all:
	make config;
	make generate;

.PHONY: visualize-data-model
# Visualize data model
visualize-data-model:
	xdg-open internal/data/ent/schema-viz.html

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
