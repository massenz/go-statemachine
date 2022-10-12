# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# Created by M. Massenzio, 2022-03-14

bin := build/bin
out := $(bin)/sm-server
tag := $(shell ./get-tag)
image := massenz/statemachine
module := $(shell go list -m)

compose := docker/docker-compose.yaml
dockerfile := docker/Dockerfile

# Source files & Test files definitions
#
# Edit only the packages list, when adding new functionality,
# the rest is deduced automatically.
#
pkgs := ./api ./grpc ./pubsub ./server ./storage
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
clean: ## Cleans up the binary, container image and other data
	@rm -f $(out)
	@docker-compose -f $(compose) down
	@docker rmi $(shell docker images -q --filter=reference=$(image))

.PHONY: build test container cov clean fmt
fmt: ## Formats the Go source code using 'go fmt'
	@go fmt $(pkgs) ./cmd ./clients

##@ Development
$(out): cmd/main.go $(srcs)
	go build -ldflags "-X $(module)/server.Release=$(tag)" -o $(out) cmd/main.go
	@chmod +x $(out)

build: $(out) ## Builds the server

test: $(srcs) $(test_srcs) services queues ## Runs all tests, starting services first, if required
	ginkgo $(pkgs)

cov: $(srcs) $(test_srcs)  ## Runs the Test Coverage target and opens a browser window with the coverage report
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

##@ Container Management
# Convenience targets to run locally containers and
# setup the test environments.
#
.PHONY: container
container: $(out) ## Builds the container image
	docker build -f $(dockerfile) -t $(image):$(tag) .

# TODO: will be replaced once we adopt TestContainers (#26)
.PHONY: services
services: ## Starts the Redis and LocalStack containers
	@docker-compose -f $(compose) up -d

.PHONY: queues
queues: ## Creates the SQS Queues in LocalStack
	@for queue in events notifications acks; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 \
			--region us-west-2 \
 			sqs create-queue --queue-name $$queue; done >/dev/null
