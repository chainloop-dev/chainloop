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
run: migration_apply
	go run ./cmd/... --conf ./configs

local_migrations_dir = file://internal/data/ent/migrate/migrations
local_db = postgres://postgres:@localhost:5432/controlplane?sslmode=disable

.PHONY: migration_apply
# run migrations against local db
migration_apply:
	atlas migrate status --dir ${local_migrations_dir} --url ${local_db}
	atlas migrate apply --dir ${local_migrations_dir} --url ${local_db}

.PHONY: migration_new
# generate new migration if needed from the current ent schema
migration_new:
	atlas migrate diff --dir ${local_migrations_dir} --to "ent://internal/data/ent/schema" --dev-url "docker://postgres/15/test?search_path=public"

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
