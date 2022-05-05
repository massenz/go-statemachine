# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

out = ${GOPATH}/bin/sm-server

api/statemachine.pb.go:
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

protos: api/statemachine.pb.go

all: protos cmd/server.go
	go build -o $(out) cmd/server.go
	@chmod +x $(out)

# TODO: Add dependencies for test files.
test: all
	@docker-compose up -d redis
	ginkgo -p ./api ./server ./storage

# Runs test coverage and displays the results in browser
cov: all
	@go test -coverprofile=/tmp/cov.out ./api ./server ./storage
	@go tool cover -html=/tmp/cov.out

clean:
	@rm -f api/*.pb.go
	@rm -f $(out)

# This is just an example to show how to run the server.
# Runs the server on http://localhost:8089 in debug mode.
run: all test
	@docker-compose up -d redis
	$(out) --port 8089 --local --debug \
		--redis localhost:6379 --sqs test-sm
