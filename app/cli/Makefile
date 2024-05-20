VERSION=$(shell git describe --tags --always)

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags \
    		  "-X github.com/chainloop-dev/chainloop/app/cli/cmd.Version=$(VERSION) \
    		  -X github.com/chainloop-dev/chainloop/app/cli/cmd.posthogAPIKey=${POSTHOG_API_KEY} \
    		  -X github.com/chainloop-dev/chainloop/app/cli/cmd.posthogEndpoint=${POSTHOG_ENDPOINT}" \
    		  -o ./bin/chainloop ./main.go

.PHONY: test
# test
test:
	go test ./...

.PHONY: lint
# lint
lint:
	golangci-lint run

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