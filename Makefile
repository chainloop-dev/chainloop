include common.mk

VERSION=$(shell git describe --tags --always)
API_PROTO_FILES=$(shell find api -name *.proto)

.PHONY: api
# generate api proto
api:
	cd ./internal/attestation/crafter/api && buf generate
	make -C ./app/controlplane api
	make -C ./app/artifact-cas api

.PHONY: config
# generate config proto
config:
	cd ./pkg/credentials/api && buf generate
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
	# make -C ./extras/dagger lint

.PHONY: test
# All tests, both unit and integration
test:
	go test ./...

.PHONY: build_devel
# build for development testing
build_devel:
	goreleaser build --snapshot --clean --single-target
.PHONY: build_devel_container_mages
# build container images for development testing
build_devel_container_mages:
	goreleaser release --clean --snapshot --skip-sign --skip-sbom