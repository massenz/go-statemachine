# Copyright (c) 2022-2024 AlertAvert.com.  All rights reserved.
# Created by M. Massenzio, 2022-03-14

GOOS ?= $(shell uname -s | tr "[:upper:]" "[:lower:]")
GOARCH ?= $(shell uname -m)
ifeq ($(GOARCH),x86_64)
	GOARCH=amd64
endif
GOMOD := $(shell go list -m)

# Versioning
# The `version` is a static value, set in settings.yaml, and ONLY used to tag the release.
VERSION ?= $(shell cat settings.yaml | yq -r .version)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
RELEASE := v$(VERSION)-g$(GIT_COMMIT)

prog := $(shell cat settings.yaml | yq -r .name)
bin := $(prog)-$(RELEASE)_$(GOOS)-$(GOARCH)

# Source files & Test files definitions

pkgs := $(shell find pkg -mindepth 1 -type d)
all_go := $(shell for d in $(pkgs); do find $$d -name "*.go"; done)
test_srcs := $(shell for d in $(pkgs); do find $$d -name "*_test.go"; done)
srcs := $(filter-out $(test_srcs),$(all_go))

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
img=$(shell docker images -q --filter=reference=$(image))
clean: ## Cleans up the binary, container image and other data
	@rm -rf build/*
	@find . -name "*.out" -exec rm {} \;
	@[ -n "$(img)" ] && docker rmi $(img) || true
	@rm -rf certs

version: ## Displays the current version tag (release)
	@echo v$(VERSION)

fmt: ## Formats the Go source code using 'go fmt'
	@go fmt $(pkgs) ./cmd fsm-cli/client fsm-cli/cmd

##@ Development
.PHONY: build test container cov clean fmt
$(out)/$(server): cmd/main.go $(srcs)
	@mkdir -p $(out)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X $(GOMOD)/pkg/api.Release=$(release)" \
		-o $(out)/$(server) cmd/main.go

.PHONY: build
build: $(out)/$(server) ## Builds the Statemachine server binary

##@ Development
.PHONY: build
build: cmd/main.go $(srcs)
	@mkdir -p build/bin
	@echo "Building rel. $(RELEASE); OS/Arch: $(GOOS)/$(GOARCH) - Pkg: $(GOMOD)"
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X $(GOMOD)/api.Release=$(RELEASE)" \
		-o build/bin/$(bin) cmd/main.go
	@echo "Majordomo Server $(shell basename $(bin)) built"

.PHONY: test
test: $(srcs) $(test_srcs)  ## Runs all tests
	@mkdir -p build/reports
	ginkgo -keepGoing -cover -coverprofile=coverage.out -outputdir=build/reports $(pkgs)
    # Clean up the coverage files (they are not needed once the
    # report is generated)
    # FIXME: once we move packages to pkg/ adjust the path to ./pkg
	@find . -name "coverage.out" -exec rm {} \;

.PHONY: watch
watch: $(srcs) $(test_srcs)  ## Runs all tests every time a source or test file changes
	ginkgo watch -p $(pkgs)

build/reports/coverage.out: test ## Runs all tests and generates the coverage report

.PHONY: coverage
coverage: build/reports/coverage.out ## Shows the coverage report in the browser
	@go tool cover -html=build/reports/coverage.out

.PHONY: all
all: build test ## Builds the binary and runs all tests

PORT ?= 7398
.PHONY: dev
dev: build ## Runs the server binary in development mode
    # FIXME: this currently does not work and should be adjusted
	build/bin/$(bin) -debug -grpc-port $(PORT)

##@ Container Management
# Convenience targets to run locally containers and
# setup the test environments.
image := massenz/$(prog)
compose := docker/compose.yaml
dockerfile := docker/Dockerfile

.PHONY: container
container: build/bin/$(bin) ## Builds the container image
	docker build -f $(dockerfile) \
		--build-arg="VERSION=$(VERSION)" \
		-t $(image):$(RELEASE) .

.PHONY: start
start: ## Runs the container locally
	@RELEASE=$(RELEASE) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up redis localstack -d
	@sleep 3
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 \
			--region us-west-2 \
 			sqs create-queue --queue-name $$queue; done
	@RELEASE=$(RELEASE) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up server

.PHONY: stop
stop: ## Stops the running containers
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm down


##@ TLS Support
#
# This section is WIP and subject to change

config_dir := ssl-config
ca-csr := $(config_dir)/ca-csr.json
ca-config := $(config_dir)/ca-config.json
server-csr := $(config_dir)/localhost-csr.json

.PHONY: gencert
gencert: $(ca-csr) $(config) $(server-csr) ## Generates all certificates in the certs directory (requires cfssl, see https://github.com/cloudflare/cfssl#installation)
	cfssl gencert \
		-initca $(ca-csr) | cfssljson -bare ca
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=$(ca-config) \
		-profile=server \
		$(server-csr)  | cfssljson -bare server
	@mkdir -p certs
	@mv *.pem certs/
	@rm *.csr
	@chmod a+r certs/*
	@echo "Certificates generated in $(shell pwd)/certs"

.PHONY: clean-cert
clean-cert:
	@rm -rf certs

# CLI Configuration
cli := out/bin/fsm-cli-$(VERSION)_$(GOOS)-$(GOARCH)
cli_config := ${HOME}/.fsm
.PHONY: cli
cli: fsm-cli/cmd/main.go  ## Builds the CLI client used to connect to the server
	@mkdir -p $(out)
	cd fsm-cli && GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X main.Release=$(RELEASE)" \
		-o ../$(out)/$(cli) cmd/main.go

.PHONY: cli-test
cli-test: ## Run tests for the CLI Client
	@mkdir -p $(cli_config)/certs
	@cp certs/ca.pem $(cli_config)/certs || true
	cd fsm-cli && RELEASE=$(RELEASE) BASEDIR=$(shell pwd) \
		CLI_TEST_COMPOSE=$(shell pwd)/docker/cli-test-compose.yaml \
		ginkgo test ./client
