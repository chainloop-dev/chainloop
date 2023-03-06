VERSION=$(shell git describe --tags --always)

.PHONY: init
# init env
init:
	go install github.com/spf13/cobra-cli@latest

# If the first argument is "run"...
ifeq (run,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

.PHONY: api
# generate api proto
api:
	cd ./api && buf generate

.PHONY: run
# run
run:
	go run main.go --insecure --control-plane localhost:9000 --artifact-cas localhost:9001 $(RUN_ARGS)

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X github.com/chainloop-dev/chainloop/app/cli/cmd.Version=$(VERSION)" -o ./bin/chainloop ./main.go

.PHONY: all
# generate all
all:
	make build;

.PHONY: test
# test
test:
	go test ./...

.PHONY: lint
# lint
lint:
	golangci-lint run
	buf lint api

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