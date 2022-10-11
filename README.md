# go-statemachine

A basic implementation of a Finite State Machine in Go

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

[![Build](https://github.com/massenz/go-statemachine/actions/workflows/build.yml/badge.svg)](https://github.com/massenz/go-statemachine/actions/workflows/build.yml)
[![Release](https://github.com/massenz/go-statemachine/actions/workflows/release.yaml/badge.svg)](https://github.com/massenz/go-statemachine/actions/workflows/release.yaml)

### Copyright & Licensing

**The code is copyright (c) 2022 AlertAvert.com. All rights reserved**<br>
The code is released under the Apache 2.0 License, see `LICENSE` for details.

Fixes and additions are always welcome and warmly appreciated, please see the [Contributing](#contributing) section for some guidance.

# Overview

## Design

The overall architecture is shown below:

![Architecture](docs/images/statemachine.png)

*System Architecture*

The REST API is described [here](#api), the Protobuf messages and gRPC methods are described in [their respective repository](https://github.com/massenz/statemachine-proto), and how [to run the server](#running-the-server) is further below.

The general design approach here optimizes for simplicity and efficiency: we would expect a single instance of the `sm-server` and a relatively low-scale Redis cluster to be able to handle millions of "statemachines" and several thousand events per second.

A "statemachine" is any business entity (uniquely identified by its `id`) whose `state` we need to track across `transitions`, where each transition is driven by an `event` - both states and events are simply described by (possibly, opaque) strings, to which the `sm-server` attaches no meaning (other than what a `configuration` defines).

A `configuration` is an immutable, versioned declaration of the `states` an FSM will subsequently traverse, along with the respective `transitions`, the latter defined as a tuple of:

```
{from, to, event}
```

Thus given the following `configuration`:

```
c1: {
  states: [s1, s2, s3],
  transitions: [ {s1, s2, evA}, {s1, s3, evB}],
  starting_state: s1
}
```
states that an FSM whose `config_id` is `c1`, will start its lifecycle in state `s1` and will end up in `s3` upon receiving `e1`:

```
e1: {event: "evB"}
```

See [Sending Events](#sending-events) below for details on how to encode an SQS Message to encode an `event`.


## API

The HTTP server exposes a REST API that allows to create (`POST`) and retrieve (`GET`) both `configurations` and `statemachines`, encoding their contents using JSON.

### Configurations

Before creating an FSM, you need to define the associated configuration (trying to create an FSM with a `configuration_version` that does not match an existing `configuration` will result in a `404 NOT FOUND` error).

> `TODO`: we may eventually choose to return a more descriptive error code (e.g., either a `403 FORBIDDEN` or a `406 METHOD NOT ALLOWED` in future)

To create a new configuration use:

```
POST /api/v1/configurations

{
  "name": "test.orders",
  "version": "v3",
  "states": [
    "start",
    "waiting",
    "pending",
    "shipped",
    "delivered",
    "completed",
    "closed"
  ],
  "transitions": [
    {
      "from": "start",
      "to": "pending",
      "event": "accept"
    },
    {
      "from": "start",
      "to": "waiting",
      "event": "pause"
    },    {
      "from": "pending",
      "to": "shipped",
      "event": "ship"
    },
    {
      "from": "shipped",
      "to": "delivered",
      "event": "deliver"
    },
    {
      "from": "delivered",
      "to": "completed",
      "event": "sign"
    },
    {
      "from": "completed",
      "to": "closed",
      "event": "close"
    }
  ],
  "starting_state": "start"
}
```

You **cannot specify an ID**, one will be created automatically by the server, using both the `name` and `version` and returned in the `Location` header:

```
201 CREATED

Location: /api/v1/configurations/test.orders:v3
```

Configurations are deemed to be immutable, so no `PUT` is offered, and also trying to re-create a configuration with the same `{name, version}` tuple will result in a `409 CONFLICT` error.

Configurations can be retrieved using the `GET` and endpoint returned:

```
GET /api/v1/configurations/test.orders:v3

{
    "name": "test.orders",
    "version": "v3",
    "states": [
        "start",
        ...
        "closed"
    ],
    "transitions": [
        ...
    ],
    "startingState": "start"
}
```

### State Machines

To create a `statemachine` simply requires indicating its [configuration](#configurations) and an (optional) ID:

```
POST /api/v1/statemachines

{
  "configuration_version": "devices:v3",
  "id": "6b5af0e8-9033-47e2-97db-337476f1402a"
}
```

if the optional `id` is omitted, one will be generated and returned in the `Location` header (as well as in the body of the response):

```
Location: /api/v1/statemachines/devices/6b5af0e8-9033-47e2-97db-337476f1402a

{
    "id": "6b5af0e8-9033-47e2-97db-337476f1402a",
    "statemachine": {
        "config_id": "devices:v3",
        "state": "started"
    }
}
```

> **Note**
> The "type" of FSM (in other words, its configuration - but **not** the version)
> is included in the FSM's URI.

To obtain the current state of the FSM, simply use a `GET`:

```
GET /api/v1/statemachines/devices/6b5af0e8-9033-47e2-97db-337476f1402a

200 OK

{
    "id": "6b5af0e8-9033-47e2-97db-337476f1402a",
    "statemachine": {
        "config_id": "devices:v3",
        "state": "backorderd",
        "history": [
            {
                "event_id": "258",
                "timestamp": {
                    "seconds": 1661733324,
                    "nanos": 461000000
                },
                "transition": {
                    "from": "started",
                    "to": "backorderd",
                    "event": "backorder"
                },
                "originator": "SimpleSender"
            }
        ]
    }
}
```

which shows that an event `backorder` was sent at `Sun Aug 28 2022 17:35:24 PDT` (the `timestamp` in seconds from epoch) transitioning our device order to a `backordered` state.

Again, the "type" of FSM **must** be specified in the URL (`/devices`).

See [`grpc_client`](clients/grpc_client.go) for a fully worked out example as to how to send events to an FSM.

### Event Outcomes

After [sending an event](#sending-events), the outcome of the event can be retrieved using the `event_id` (either specified, or auto-generated by the server):

```
GET /api/v1/events/outcome/orders/f8b6a19b-12c9-40b1-aa35-240cd829b014

{
    "status_code": "Ok",
    "message": "event [accept] transitioned FSM [25] to state [pending]",
    "destination": "orders#6b5af0e8-9033-47e2-97db-337476f1402a"
}
```

if there was an error, it would be reported too, with the relevant message, if available:

```
GET /api/v1/events/outcome/test.orders/4018047a-50c1-45ea-b87e-e79b195568db

{
    "status_code": "TransitionNotAllowed",
    "message": "event [self-destroy] could not be processed: unexpected event transition",
    "destination": "test.orders#6b5af0e8-9033-47e2-97db-337476f1402a"
}
```

Note that, as for FSMs, we need to qualify the `event_id` with the `configuration.name` in the URL (in this example `test.orders`).

## Sending Events

> Note that **it is not possible to send events via the REST API**: this is **by design** and not just a "missing feature"; please do not submit requests to add a `POST /api/v1/events` API: it's not going to happen.

### SQS Messages

#### EventRequest

To send an Event to an FSM via an SQS Message we use the [following code](clients/sqs_client.go):

```golang
// This is the object you want to send across as Event's metadata.
order := NewOrderDetails(uuid.NewString(), "sqs-cust-1234", 99.99)

msg := &protos.EventRequest{
    Event: &protos.Event{
        // This is actually unnecessary; if no EventId is present, SM will
        // generate one automatically and if the client does not need to store
        // it somewhere else, it is safe to omit it.
        EventId:    uuid.NewString(),

        // This is also unnecessary, as SM will automatically generate a timestamp
        // if one is not already present.
        Timestamp:  timestamppb.Now(),

        Transition: &protos.Transition{Event: "backorder"},
        Originator: "New SQS Client with Details",

        // Here you convert the Event metadata to a string by, e.g., JSON-serializing it.
        Details:    order.String(),
    },

    // This is the unique ID for the entity you are sending the event to; MUST
    // match the `id` of an existing `statemachine` (see the REST API).
    // NOTE -- the ID is prefixed by the configuration name.
    Dest: "devices#6b5af0e8-9033-47e2-97db-337476f1402a",
}

_, err = queue.SendMessage(&sqs.SendMessageInput{
    // Here we serialize the Protobuf using text serialization.
    MessageBody: aws.String(proto.MarshalTextString(msg)),
    QueueUrl:    queueUrl.QueueUrl,
})
```

This will cause a `backorder` event to be sent to our FSM whose `id` matches the UUID in `Dest`; if there are errors (eg, the FSM does not exist, or the event is not allowed for the machine's configuration and current state) errors may be optionally sent to the SQS queue configured via the `-notifications` option (see [Running the Server](#running-the-server)): see the [`pubsub` code](pubsub/sqs_pub.go) code for details as to how we encode the error message as an SQS message.

See [`EventRequest` in `statemachine-proto`](https://github.com/massenz/statemachine-proto/blob/main/api/statemachine.proto#L86) for details on the event being sent.

To try this out, you can use the [`SQS Client`](clients/sqs_client.go) example:

```
└─( http POST :7399/api/v1/statemachines configuration_version=test.orders:v3 id=26
HTTP/1.1 201 Created
Location: /api/v1/statemachines/test.orders/26

{
    "id": "26",
    "statemachine": {
        "config_id": "test.orders:v3",
        "state": "start"
    }
}

└─( SQS_Client -endpoint http://localhost:4566 -q events \
    -dest test.orders#26 -evt ship

Publishing Event `ship` for FSM `test.orders#26` to SQS Topic: [events]
Sent event [724ea354-4739-4782-8785-6ce55b86a25d] to queue events

└─( http :7399/api/v1/events/outcome/test.orders/724ea354-4739-4782-8785-6ce55b86a25d
HTTP/1.1 200 OK

{
    "destination": "test.orders#26",
    "message": "event [ship] transitioned FSM [25] to state [shipped]",
    "status_code": "Ok"
}
```


#### SQS Notifications

Event processing outcomes are returned in [`EventResponse` protocol buffers](https://github.com/massenz/statemachine-proto/blob/main/api/statemachine.proto#L112), which are then serialized inside the `body` of the SQS message; to retrieve the actual Go struct, you can use code such as this (see [test code](pubsub/sqs_pub_test.go#L148) for actual working code):

```go
// `res` is what AWS SQS Client will return to the Messages slice
var res *sqs.Message = getSqsMessage(getQueueName(notificationsQueue))
var receivedEvt protos.EventResponse
err := proto.UnmarshalText(*res.Body, &receivedEvt)
if err == nill {
    // you know what to do
}

receivedEvt.EventId --> is the ID of the Event that failed
if receivedEvt.Outcome.Code == protos.EventOutcome_InternalError {
    // whatever...
}
return fmt.Errorf("cannot process event to statemachine [%s]: %s,
    receivedEvt.Outcome.Dest, receivedEvt.Outcome.Details)

```

The possible error codes are (see the `.proto` definition for the up-to-date values):

```proto
  enum StatusCode {
    Ok = 0;
    GenericError = 1;
    EventNotAllowed = 2;
    FsmNotFound = 3;
    TransitionNotAllowed = 4;
    InternalError = 5;
    MissingDestination = 6;
    ConfigurationNotFound = 7;
  }
```

### gRPC Methods

All the actions described above in [the API](#api) section can also be executed via gRPC method calls.

Please refer to [gRPC documentation](https://grpc.io/docs/), the [example gRPC client](clients/grpc_client.go) and [the Protocol Buffers repository](https://github.com/massenz/statemachine-proto) for more information and details as to how to send events using the `ConsumeEvent()` API.

The TL;DR version of all the above is that code like this:

```go
response, err := client.ProcessEvent(context.Background(),
    &api.EventRequest{
        Event: &api.Event{
            EventId:   uuid.NewString(),
            Timestamp: timestamppb.Now(),
            Transition: &api.Transition{
                Event: "backorder",
            },
            Originator: "gRPC Client",
        },
        Dest: "6b5af0e8-9033-47e2-97db-337476f1402a",
    })
```

like in the SQS example, will cause a `backorder` event to be sent to our FSM whose `id` matches the UUID in `dest`; the `response` message will contain either the ID of the event, or a suitable error will be returned.

**NOTE**<br/>
As the event processing is asynchronous, the `response` will only contain the `event_id` and an empty `outcome`; use the `event_id` and the `GetEventOutcome(GetRequest)` to retrieve the result of event processing (or directly fetch the FSM via the `GetStatemachine()` method to confirm it has transitioned to the expected state - or not).


# Build & Run

## Prerequisites

**Ginkgo testing framework**<br/>
We are using [Ginkgo](https://onsi.github.io/ginkgo/) at **v1** (`v1.16.5`).

To install the CLI run this:

    go install github.com/onsi/ginkgo/ginkgo

> **Beware** Gingko now is at `v2` and will install that one by default if you follow the instruction on the site: use instead the command above and run `go mod tidy` before running the tests/builds to download packages<br/>
> (see [this issue](https://github.com/onsi/ginkgo/issues/945) for more details)


**Protocol Buffers definitions**<br/>
They are kept in the [statemachine-proto](https://github.com/massenz/statemachine-proto) repository; nothing specific is needed to use them; however, if you want to review the messages and services definitions, you can see them there.

**Supporting services**<br/>
The `sm-server` requires a running [Redis](#) server and [AWS Simple Queue Service (SQS)](#); they can be both run locally in containers: see `docker/docker-compose.yaml` and [Container Build & Run](#container-build--run).


## Build & Test

The `sm-server` is built with

    make build

and the tests are run with `make test`.

The binary is in `build/bin` and to see all the available configuration options use:

        build/bin/sm-server -h

Prior to running the server, if you want to use the local running stack, use:

        make services && make queues

To create the necessary SQS Queues in AWS, please see the `aws` CLI command in `Makefile`, `queues` recipe, using a valid profile (in `AWS_PROFILE`) and Region (`AWS_REGION`), with the required IAM permissions.

## Running the Server

The `sm-server` accepts a number of configuration options (some of them are **required**):

```
└─( build/bin/sm-server -help

Usage of build/bin/sm-server:
  -debug
    	Verbose logs; better to avoid on Production services
  -endpoint-url string
    	HTTP URL for AWS SQS to connect to; usually best left undefined, unless required for local testing purposes (LocalStack uses http://localhost:4566)
  -events string
    	If defined, it will attempt to connect to the given SQS Queue to receive events from the Pub/Sub system
  -grpc-port int
    	The port for the gRPC server (default 7398)
  -http-port int
    	HTTP Server port for the REST API (default 7399)
  -local
    	If set, it only listens to incoming requests from the local host
  -max-retries int
    	Max number of attempts for a recoverable error to be retried against the Redis cluster (default 3)
  -notifications string
    	The name of the notification topic in SQS to publish events' outcomes to; if not specified, no outcomes will be published
  -acks string
    	(Requires `notifications`) The name of the acks topic in SQS to publish events' outcomes to; if specified with a value, Ok outcomes will be published to the acks topic and other (error) outcomes to the notification topic. If specified without a value, Ok outcomes will not be published
  -redis string
    	URI for the Redis cluster (host:port)
  -cluster
        Enables connecting to a Redis deployment in cluster mode
  -timeout duration
    	Timeout for Redis (as a Duration string, e.g. 1s, 20ms, etc.) (default 200ms)
  -trace
    	Enables trace logs for every API request and Pub/Sub event; it may impact performance, do not use in production or on heavily loaded systems (will override the -debug option)
```

the easiest way is to run it [as a container](#container-build--run) (see also **Supporting Services** in [Prerequisites]](#prerequisites)):

```
tag=$(./get-tag)
make container
docker run --rm -d -p 7399:7399 --name sm-server \
    --env AWS_ENDPOINT=http://awslocal:4566 --env TIMEOUT=200ms \
    --env DEBUG=-debug --network docker_sm-net  \
      massenz/statemachine:${tag}
```

If you want to connect it to an actual AWS account, configure your AWS credentials appropriately, and use `AWS_PROFILE` if not using the `default` account:

`AWS_PROFILE=my-profile AWS_REGION=us-west-2 build/bin/sm-server -debug -events events`

will try and connect to an SQS queue named `events` in the `us-west-2` region.

The server will expose both an HTTP REST API (on the `-http-port` defined) and a gRPC server (listening on the `-grpc-port`).

For an example of how to send events either to an SQS queue or via a gRPC call, see example clients in the [`clients`](clients) folder.

Logs are sent to `stdout` by default, but this can be changed using the [`slf4go`](https://github.com/massenz/slf4go) configuration methods.


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


# Contributing

Please follow the Go Style enshrined in `go fmt` before submitting PRs, refer to actual [Issues](https://github.com/massenz/go-statemachine/issues), and provide sufficient testing (ideally, ensuring code coverage is better than 70%).

We prefer you submit a PR directly from cloning this repository and creating a feature branch, rather than from a fork.

If you are relatively new to this topic, there are a few issues labeled as [`Good First Issue`](https://github.com/massenz/go-statemachine/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22), those are a good starting point.

Please, always **squash and rebase** on `main`, we try to keep a relatively clean and linear commit history, and each PR should just be **one** commit with a descriptive title (see below).

## Detailed Requirements to submit a PR

1. make sure the Issue you are trying to address is referenced appropriately in the PR Title:

```
[#34] Implements gRPC API to update/retrieve Event outcomes
```

2. prior to submitting the PR, make sure that the code is properly formatted and tested:

```
make fmt && make test
```

3. if this is a breaking change, or a significant change to the API or behavior, please update the [README](README.md) accordingly.

## Guidelines to be A Good Contributor

Provide enough detail of your changes in the PR comments and make it easy for reviewers:

* if your code contains several lines of "commented out dead code" make sure that you clearly explain why this is so with a `TODO` and an explanation of why are you leaving dead code around (remember, we are using `git` here, there is no such thing "in case we forget" - `git` **never** forgets)
* try and be consistent with the rest of the code base and, specifically, the code around the changes you are implementing
* be consistent with the `import` format and sequence: if in doubt, again, look at the existing code and be **consistent**
* make sure the new code is **covered by unit tests**, use `make cov` to check coverage % and view lines covered in the browser
* try and adopt "The Boyscout Rule": leave the campsite cleaner than you found it -- in other words, adding tests, fixing typos, fixing **minor** issues is always **greatly** appreciated
* conversely, try and keep the PR focused on one topic/issue/change, and one only: we'd rather review 2 PRs, than try and disentangle the two unrelated issues you're trying to address

If in doubt, look at the existing code, or feel free to ask in the PR's comments - we don't bite :-)
