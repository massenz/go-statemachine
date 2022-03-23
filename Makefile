# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

protos:
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

build: protos
	go build -o bin/sm-server cmd/server.go
	chmod +x bin/sm-server

test: build
	ginkgo ./api ./server

# Runs the server on http://localhost:8089 in debug mode.
run: build test
	bin/sm-server --port 8089 --local --debug

# Runs test coverage and displays the results in browser
cov: build
	go test -coverprofile=/tmp/cov.out ./api ./server
	go tool cover -html=/tmp/cov.out
