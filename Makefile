# Copyright (c) 2022-2024 AlertAvert.com.  All rights reserved.
# Created by M. Massenzio, 2022-03-14

include thirdparty/common.mk
bin := $(appname)-$(release)_$(GOOS)-$(GOARCH)

# Source files & Test files definitions
pkgs := $(shell find pkg -mindepth 1 -type d)
all_go := $(shell for d in $(pkgs); do find $$d -name "*.go"; done)
test_srcs := $(shell for d in $(pkgs); do find $$d -name "*_test.go"; done)
srcs := $(filter-out $(test_srcs),$(all_go))

##@ General
.PHONY: clean
img=$(shell docker images -q --filter=reference=$(image))
clean: clean-cert  ## Cleans up the binary, container image and other data
	@rm -rf build/*
	@find . -name "*.out" -exec rm {} \;
ifneq (,$(img))
	@docker rmi $(img) || true
endif

version: ## Displays the current version tag (release)
	@echo v$(version)

fmt: ## Formats the Go source code using 'go fmt'
	@go fmt $(pkgs) ./cmd fsm-cli/client fsm-cli/cmd

##@ Development
.PHONY: build
build: cmd/main.go $(srcs)  ## Builds the binary
	@mkdir -p build/bin
	@echo "$(GREEN)Building rel. $(release)$(RESET); OS/Arch: $(GOOS)/$(GOARCH) - Pkg: $(GOMOD)"
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X $(GOMOD)/api.Release=$(release)" \
		-o build/bin/$(bin) cmd/main.go
	@echo "$(GREEN)[SUCCESS]$(RESET) Binary $(shell basename $(bin)) built"

build/bin/$(bin): build

.PHONY: test
test: $(srcs) $(test_srcs)  check_certs  ## Runs all tests
	@mkdir -p build/reports
	ginkgo -keepGoing -cover -coverprofile=coverage.out -outputdir=build/reports $(pkgs)
    # Clean up the coverage files (they are not needed once the
    # report is generated)
	@find ./pkg -name "coverage.out" -exec rm {} \;

.PHONY: watch
watch: $(srcs) $(test_srcs)  ## Runs all tests every time a source or test file changes
	ginkgo watch -p $(pkgs)

build/reports/coverage.out: test ## Runs all tests and generates the coverage report

.PHONY: coverage
coverage: build/reports/coverage.out ## Shows the coverage report in the browser
	@go tool cover -html=build/reports/coverage.out

.PHONY: all
all: build gencert test ## Builds the binary and runs all tests

PORT ?= 7398
.PHONY: dev
dev: build ## Runs the server binary in development mode
    # FIXME: this currently does not work and should be adjusted
	build/bin/$(bin) -debug -grpc-port $(PORT)

##@ Container Management
# Convenience targets to run locally containers and
# setup the test environments.
image := massenz/$(appname)
compose := docker/compose.yaml
dockerfile := docker/Dockerfile

.PHONY: container
container: build/bin/$(bin) ## Builds the container image
	docker build -f $(dockerfile) \
		--build-arg="VERSION=$(version)" \
		-t $(image):$(release) .
	@echo "$(GREEN)[SUCCESS]$(RESET) Container image $(YELLOW)$(image):$(release)$(RESET) built"

.PHONY: start
start: ## Runs the container locally
	@echo "$(GREEN)[STARTING]$(RESET) Stopping containers"
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up redis localstack -d
	@sleep 3
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 \
			--region us-west-2 \
 			sqs create-queue --queue-name $$queue; done
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up server
	@echo "$(GREEN)[SUCCESS]$(RESET) Containers started"

.PHONY: stop
stop: ## Stops the running containers
	@echo "$(RED)[STOPPING]$(RESET) Stopping containers"
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm down


##@ TLS Support
#
# This section is WIP and subject to change
# Dependency checks for the 'test' target
.PHONY: check_certs
check_certs:
	@num_certs=$$(ls -1 certs/*.pem 2>/dev/null | wc -l); \
	if [ $$num_certs != 4 ]; then \
		echo "$(YELLOW)[WARN]$(RESET) No certificates found in $(shell pwd)/certs"; \
		make certs; \
		echo "$(GREEN)[SUCCESS]$(RESET) Certificates generated in $(shell pwd)/certs"; \
	else \
		echo "$(GREEN)[OK]$(RESET) Certificates found in $(shell pwd)/certs"; \
	fi

ssl_config := ../ssl-config
ca-csr := $(ssl_config)/ca-csr.json
ca-config := $(ssl_config)/ca-config.json
server-csr := $(ssl_config)/localhost-csr.json

.PHONY: certs
certs:  ## Generates all certificates in the certs directory (requires cfssl, see https://github.com/cloudflare/cfssl#installation)
	@mkdir -p certs
	@cd certs && \
		cfssl gencert \
			-initca $(ca-csr) 2>/dev/null | cfssljson -bare ca
	@cd certs && \
		cfssl gencert \
			-ca=ca.pem \
			-ca-key=ca-key.pem \
			-config=$(ca-config) \
			-profile=server \
			$(server-csr)  2>/dev/null | cfssljson -bare server
	@rm certs/*.csr
	@chmod a+r certs/*.pem
	@echo "$(GREEN)[SUCCESS]$(RESET) Certificates generated"

.PHONY: clean-cert
clean-cert:
	@rm -rf certs

##@ CLI Client
# TODO: move to a separate Makefile in the subdirectory
# See: https://www.gnu.org/software/make/manual/html_node/Recursion.html
# CLI Configuration
cli := out/bin/$(appname)-cli-$(version)_$(GOOS)-$(GOARCH)
cli_config := ${HOME}/.fsm
.PHONY: cli
cli: fsm-cli/cmd/main.go  ## Builds the CLI client used to connect to the server
	@mkdir -p build/bin
	cd fsm-cli && GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X main.Release=$(release)" \
		-o ../build/bin/$(cli) cmd/main.go

.PHONY: cli-test
cli-test: ## Run tests for the CLI Client
	@mkdir -p $(cli_config)/certs
	@cp certs/ca.pem $(cli_config)/certs || true
	cd fsm-cli && RELEASE=$(release) BASEDIR=$(shell pwd) \
		CLI_TEST_COMPOSE=$(shell pwd)/docker/cli-test-compose.yaml \
		ginkgo test ./client
