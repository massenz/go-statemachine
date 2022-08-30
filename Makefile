# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

pkgs := ./api ./grpc ./pubsub ./server ./storage
bin := build/bin
out := $(bin)/sm-server
tag := $(shell ./get-tag)
image := massenz/statemachine

compose := docker/docker-compose.yaml
dockerfile := docker/Dockerfile

build: cmd/main.go
	go build -ldflags "-X main.Release=$(tag)" -o $(out) cmd/main.go
	@chmod +x $(out)

services:
	@docker-compose -f $(compose) up -d

queues:
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 --region us-west-2 \
 			sqs create-queue --queue-name $$queue; done >/dev/null

test: build services queues
	ginkgo -p $(pkgs)

container:
	docker build -f $(dockerfile) -t $(image):$(tag) .

# Runs test coverage and displays the results in browser
cov: build services queues
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

clean:
	@rm -f api/*.pb.go $(out)
	@docker-compose -f $(compose) down
