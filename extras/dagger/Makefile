.PHONY: module-init
module-init:
	# Generate dagger.gen.go and querybuilder required files 
	dagger develop

.PHONY: lint
lint: check-dagger-tool module-init
	cd ./src && golangci-lint run

.PHONY: check-dagger-tool
check-dagger-tool:
	@if ! command -v dagger >/dev/null 2>&1; then \
		echo "dagger is not installed. Please install the tool manually."; \
		exit 1; \
	fi