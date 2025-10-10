include ../../common.mk

.PHONY: config
# generate config proto bindings
config: check-buf-tool
	cd ./internal/conf && buf generate

.PHONY: api
# generate api proto bindings
api: check-buf-tool
	cd ./plugins/sdk/v1/plugin/api && buf generate
	cd ./api && buf generate && \
	buf generate \
		--path ./controlplane/v1/referrer.proto \
		--path ./controlplane/v1/openapi_metadata.proto && \
		../scripts/modify-openapi-schema.sh ${PWD}/api/gen temp-openapi/apidocs.swagger.json openapi/openapi/openapi.yaml openapi/openapi-extra.yaml

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

local_migrations_dir = file://pkg/data/ent/migrate/migrations
local_db = postgres://postgres:@localhost:5432/controlplane?sslmode=disable

.PHONY: migration_apply
# run migrations against local db
migration_apply: check-atlas-tool migration_hash
	atlas migrate status --dir ${local_migrations_dir} --url ${local_db}
	atlas migrate apply --dir ${local_migrations_dir} --url ${local_db}

.PHONY: migration_apply_non_linear
# see https://atlasgo.io/versioned/apply#non-linear-error
migration_apply_non_linear: check-atlas-tool migration_hash
	atlas migrate status --dir ${local_migrations_dir} --url ${local_db}
	atlas migrate apply --dir ${local_migrations_dir} --url ${local_db} --exec-order non-linear

.PHONY: migration_sync
# sync migration files with the current ent schema
migration_sync: check-atlas-tool migration_hash
	atlas migrate diff --dir ${local_migrations_dir} --to "ent://pkg/data/ent/schema" --dev-url "docker://postgres/15/test?search_path=public" --config file://atlas.hcl --env dev

migration_lint: check-atlas-tool migration_hash
	atlas migrate lint --dir ${local_migrations_dir} --dev-url "docker://postgres/15/test?search_path=public" --latest 1 --config file://atlas.hcl --env dev

.PHONY: migration_rebase
# rebase migrations
# example: make migration_rebase 20250327153948
migration_rebase: check-atlas-tool
	atlas migrate rebase --dir ${local_migrations_dir} $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migration_new
# generate an empty migration file
migration_new: check-atlas-tool migration_hash
	atlas migrate new --dir ${local_migrations_dir}

.PHONY: migration_hash
migration_hash: check-atlas-tool
	atlas migrate hash --dir ${local_migrations_dir}

.PHONY: test
# All tests, both unit and integration
test: 
	go test $$(go list ./... | grep -v /pkg/data/ent)

.PHONY: test-unit
# unit testing, excluding integration tests
test-unit:
	SKIP_INTEGRATION=true make test

.PHONY: lint
# lint
lint: check-golangci-lint-tool check-buf-tool
	buf lint api
	buf lint internal/conf
	golangci-lint run

.PHONY: generate
# generate proto bindings, wire injectors, and ent models
generate: check-wire-tool api config migration_sync
	go generate ./...

.PHONY: all
# generate all
all:
	make config;
	make generate;

