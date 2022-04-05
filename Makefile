# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
# (Reluctantly) Created by M. Massenzio, 2022-03-14

api/statemachine.pb.go:
	protoc --proto_path=protos/ \
               --go_out=api/ \
               --go_opt=paths=source_relative \
               protos/*.proto

protos: api/statemachine.pb.go

build: protos
	go build -o ${GOPATH}/bin/sm-server cmd/server.go
	chmod +x ${GOPATH}/bin/sm-server

test: api/statemachine.pb.go
	docker-compose up -d redis
	ginkgo -p ./api ./server ./storage

# Runs the server on http://localhost:8089 in debug mode.
run: build test
	docker-compose up -d
	sm-server --port 8089 --local --debug \
		--redis localhost:6379
		# TODO: Implement support for Kafka
		#    --kafka localhost:9092

# Runs test coverage and displays the results in browser
cov: build
	go test -coverprofile=/tmp/cov.out ./api ./server ./storage
	go tool cover -html=/tmp/cov.out

clean:
	rm -f api/*.pb.go
	rm -f ${GOPATH}/bin/sm-server
