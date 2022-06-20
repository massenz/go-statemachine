# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

pkgs := ./api ./pubsub ./server ./storage
bin := build/bin
out := $(bin)/sm-server
tag := $(shell ./get-tag)
image := massenz/statemachine

compose := docker/docker-compose.yaml
dockerfile := docker/Dockerfile

api/statemachine.pb.go: protos/statemachine.proto
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

all: api/statemachine.pb.go cmd/server.go
	go build -o $(out) cmd/server.go
	@chmod +x $(out)

services:
	@docker-compose -f $(compose) up -d

queues:
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 --region us-west-2 \
 			sqs create-queue --queue-name $$queue; done >/dev/null

test: all services queues
	ginkgo -p $(pkgs)

container: test
	docker build -f $(dockerfile) -t $(image):$(tag) .

# Runs test coverage and displays the results in browser
cov: all services queues
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

clean:
	@rm -f api/*.pb.go $(out)
	@docker-compose -f $(compose) down
