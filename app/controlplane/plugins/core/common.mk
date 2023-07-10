# Metadata about this makefile and position
PLUGIN_PATH := $(patsubst %/v1/,%,$(dir $(realpath $(MKFILE_PATH))))
PLUGIN_NAME := $(notdir $(PLUGIN_PATH))

# Shared makefile to be used by the different plugins
# Build plugin
build:
	go build -o ../../../bin/chainloop-plugin-$(PLUGIN_NAME) ./cmd/...
.PHONY: build
