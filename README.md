# go-statemachine

A basic implementation of a Finite State Machine in Go

![Version](https://img.shields.io/badge/Version-0.2.0-blue)
![Released](https://img.shields.io/badge/unreleased-green)

[![Author](https://img.shields.io/badge/Author-M.%20Massenzio-green)](https://github.com/massenz)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![OS Debian](https://img.shields.io/badge/OS-Linux-green)

### Copyright & Licensing

**The code is copyright (c) 2022 AlertAvert.com. All rights reserved**<br>
The code is released under the Apache 2.0 License, see `LICENSE` for details.

# Usage

`TODO`

# Design

The overall architecture is shown below:

![Architecture](docs/images/statemachine.png)

*System Architecture*


# Build & Run

## Prerequisites

Before building/running the server, you will need to install `protoc`, the `protoc-gen-go` plugin and `ginkgo`; please follow the instructions below before attempting to running any of the `make` commands.

**Ginkgo testing framework**<br/>
Run this:

    go get github.com/onsi/ginkgo/v1/ginkgo &&
        go get github.com/onsi/gomega/...

**Building Protocol Buffers definitions**<br/>
All the base classes are defined in the `protos` folder and are used to (de)serialize state machines for storage in the database.

See [installation instructions](https://developers.google.com/protocol-buffers/docs/gotutorial#compiling-your-protocol-buffers) for compiling protobufs for Go.

It mostly boils down to the following:

```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

go get google.golang.org/grpc@v1.32.0
go get google.golang.org/protobuf@v1.27.1
```

**Supporting services**<br/>
The `sm-server` requires a running [Redis](#) server and [AWS Simple Queue Service (SQS)](#); they can be both run locally in containers: see `docker/docker-compose.yaml` and [Container Build & Run](#container-build--run).


## Build & Test

The `sm-server` is built with

        make

and the tests are run with `make test`.

The binary is in `build/bin` and to see all the available configuration options use:

        build/bin/sm-server -h

Prior to running the server, if you want to use the local running stack, use:

        make services && make queues

To create the necessary SQS Queues in AWS, please see the `aws` CLI command in `Makefile`, `queues` recipe, using a valid profile (in `AWS_PROFILE`) and Region (`AWS_REGION`), with the required IAM permissions.

## Container Build & Run

Running the server inside a container is much preferable; to build the container use:

        make container

and then:

        tag=$(./get-tag)
        docker run --rm -d -p 7399:7399 --name sm-server \
            --env AWS_ENDPOINT=http://awslocal:4566 \
            --env DEBUG=-debug --network docker_sm-net  \
            massenz/statemachine:${tag}

These are the environment variables whose values can be modified as necessary (see also the `Dockerfile`):

```dockerfile
ENV AWS_REGION=us-west-2
ENV AWS_PROFILE=sm-bot

# Sensible defaults for the server
# See entrypoint.sh
ENV SERVER_PORT=7399
ENV EVENTS_Q=events
ENV ERRORS_Q=notifications
ENV REDIS=redis
ENV REDIS_PORT=6379
ENV DEBUG=""
```

Additionally, a valid `credentials` file will need to be mounted (using the `-v` flag) in the container if connecting to AWS (instead of LocalStack):

        -v /full/path/to/.aws/credentials:/home/sm-bot/.aws/credentials

where the `[profile]` matches the value in `AWS_PROFILE`.
