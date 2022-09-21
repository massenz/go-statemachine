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

# Builds the server
#
$(out): cmd/main.go $(srcs)
	go build -ldflags "-X $(module)/server.Release=$(tag)" -o $(out) cmd/main.go
	@chmod +x $(out)

build: $(out)

# Convenience targets to run locally containers and
# setup the test environments.
#
# TODO: will be replaced once we adopt TestContainers
# (see Issue # 26)
services:
	@docker-compose -f $(compose) up -d

queues:
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 \
			--region us-west-2 \
 			sqs create-queue --queue-name $$queue; done >/dev/null

test: $(srcs) $(test_srcs) services queues
	ginkgo -p $(pkgs)

container: $(out)
	docker build -f $(dockerfile) -t $(image):$(tag) .

# Runs test coverage and displays the results in browser
cov: $(srcs) $(test_srcs)
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

clean:
	@rm -f $(out)
	@docker-compose -f $(compose) down
	@docker rmi $(shell docker images -q --filter=reference=$(image))
