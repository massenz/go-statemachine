# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

compile:
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

build: compile test
	go build -o bin/sm-server cmd/server.go

test: compile
	ginkgo -r
