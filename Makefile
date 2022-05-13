# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

out = ${GOPATH}/bin/sm-server
pkgs = ./api ./pubsub ./server ./storage

api/statemachine.pb.go: protos/statemachine.proto
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

all: api/statemachine.pb.go cmd/server.go
	go build -o $(out) cmd/server.go
	@chmod +x $(out)

test: all
	@docker-compose up -d
	ginkgo -p $(pkgs)

# Runs test coverage and displays the results in browser
cov: all
	@docker-compose up -d
	@go test -coverprofile=/tmp/cov.out $(pkgs)
	@go tool cover -html=/tmp/cov.out

clean:
	@rm -f api/*.pb.go $(out)

# This is just an example to show how to locally run the server.
# Runs the server on http://localhost:8089 in debug mode, using entirely
# local services (Redis and SQS).
run: all
	@docker-compose up -d
	@for queue in events notifications; do \
		aws --no-cli-pager --endpoint-url=http://localhost:4566 --region us-west-2 \
 			sqs create-queue --queue-name $$queue; done >/dev/null
	$(out) -port 8089 -local -debug \
		-redis localhost:6379 -sqs http://localhost:4566 \
		-events events -errors notifications
