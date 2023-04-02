# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# Created by M. Massenzio, 2022-03-14


GOOS ?= $(shell uname -s | tr "[:upper:]" "[:lower:]")
GOARCH ?= amd64
GOMOD := $(shell go list -m)

version := v0.11.0
release := $(version)-g$(shell git rev-parse --short HEAD)
prog := sm-server
bin := out/bin/$(prog)-$(version)_$(GOOS)-$(GOARCH)
dockerbin := out/bin/$(prog)-$(version)_linux-amd64
healthcheck := out/bin/grpc-health_linux-amd64

image := massenz/statemachine
compose := docker/compose.yaml
dockerfile := docker/Dockerfile

# Source files & Test files definitions
#
# Edit only the packages list, when adding new functionality,
# the rest is deduced automatically.
#
pkgs := ./api ./grpc ./pubsub ./storage
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
	@rm -f $(bin)
	@[ ! -z $(img) ] && docker rmi $(img) || true
	@rm -rf certs

version: ## Displays the current version tag (release)
	@echo $(release)

fmt: ## Formats the Go source code using 'go fmt'
	@go fmt $(pkgs) ./cmd ./clients

##@ Development
.PHONY: build test container cov clean fmt
$(bin): cmd/main.go $(srcs)
	@mkdir -p $(shell dirname $(bin))
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X $(GOMOD)/api.Release=$(release)" \
		-o $(bin) cmd/main.go

$(dockerbin): $(srcs)
	GOOS=linux GOARCH=amd64 go build \
		-ldflags "-X $(GOMOD)/api.Release=$(release)" \
		-o $(dockerbin) cmd/main.go

$(healthcheck): grpc_health.go
	GOOS=linux GOARCH=amd64 go build -o $(healthcheck) grpc_health.go

.PHONY: build
build: $(bin) ## Builds the Statemachine server binary

test: $(srcs) $(test_srcs)  ## Runs all tests
	ginkgo $(pkgs)

cov: $(srcs) $(test_srcs)  ## Runs the Test Coverage target and opens a browser window with the coverage report
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

##@ Container Management
# Convenience targets to run locally containers and
# setup the test environments.

container: $(dockerbin) $(healthcheck) ## Builds the container image
	docker build --build-arg appname=$(dockerbin) \
		--build-arg hc=$(healthcheck) \
		-f $(dockerfile) -t $(image):$(release) .

.PHONY: start
start: ## Starts the Redis and LocalStack containers, and Creates the SQS Queues in LocalStack
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up redis localstack -d
	@sleep 3
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 \
			--region us-west-2 \
 			sqs create-queue --queue-name $$queue; done
	@RELEASE=$(release) BASEDIR=$(shell pwd) docker compose -f $(compose) --project-name sm up server

.PHONY: stop
stop: ## Stops the Redis and LocalStack containers
	@docker compose -f $(compose) --project-name sm down


##@ TLS Support
#
# This section is WIP and subject to change

config_dir := ssl-config
ca-csr := $(config_dir)/ca-csr.json
ca-config := $(config_dir)/ca-config.json
server-csr := $(config_dir)/localhost-csr.json

.PHONY: gencert
gencert: $(ca-csr) $(config) $(server-csr) ## Generates all certificates in the certs directory (requires cfssl and cfssl, see https://github.com/cloudflare/cfssl#installation)
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
	@echo "Certificates generated in $(shell pwd)/certs"

.PHONY: clean-cert
clean-cert:
	@rm -rf certs
