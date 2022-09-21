/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

// CLI to process Kubernetes Specs with a JSON configuration.
//
// Created by M. Massenzio, 2021-02-20

package main

import (
    "flag"
    "fmt"
    "github.com/massenz/go-statemachine/grpc"
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/go-statemachine/server"
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "net"
    "sync"
)

func SetLogLevel(services []log.Loggable, level log.LogLevel) {
    for _, s := range services {
        if s != nil {
            s.SetLogLevel(level)
        }
    }
}

var (
    logger                      = log.NewLog("sm-server")
    serverLogLevel log.LogLevel = log.INFO

    host = "0.0.0.0"

    store storage.StoreManager

    sub      *pubsub.SqsSubscriber
    pub      *pubsub.SqsPublisher = nil
    listener *pubsub.EventsListener

    // TODO: for now blocking channels; we will need to confirm
    //  whether we can support a fully concurrent system with a
    //  buffered channel
    errorsCh chan protos.EventResponse = nil
    eventsCh                           = make(chan protos.EventRequest)

    wg sync.WaitGroup
)

func main() {
    defer close(eventsCh)

    var debug = flag.Bool("debug", false,
        "Verbose logs; better to avoid on Production services")
    var trace = flag.Bool("trace", false,
        "Enables trace logs for every API request and Pub/Sub event; it may impact performance, "+
            "do not use in production or on heavily loaded systems ("+
            "will override the -debug option)")
    var localOnly = flag.Bool("local", false,
        "If set, it only listens to incoming requests from the local host")
    var port = flag.Int("http-port", 7399, "HTTP Server port for the REST API")
    var redisUrl = flag.String("redis", "", "URI for the Redis cluster (host:port)")
    var awsEndpoint = flag.String("endpoint-url", "",
        "HTTP URL for AWS SQS to connect to; usually best left undefined, "+
            "unless required for local testing purposes (LocalStack uses http://localhost:4566)")
    var eventsTopic = flag.String("events", "", "If defined, it will attempt to connect "+
        "to the given SQS Queue (ignores any value that is passed via the -kafka flag)")
    var dlqTopic = flag.String("errors", "",
        "The name of the Dead-Letter Queue ("+"DLQ) in SQS to post errors to; if not "+
            "specified, the DLQ will not be used")
    var grpcPort = flag.Int("grpc-port", 7398, "The port for the gRPC server")
    var maxRetries = flag.Int("max-retries", storage.DefaultMaxRetries,
        "Max number of attempts for a recoverable error to be retried against the Redis cluster")
    var timeout = flag.Duration("timeout", storage.DefaultTimeout,
        "Timeout for Redis (as a Duration string, e.g. 1s, 20ms, etc.)")
    flag.Parse()

    logger.Info("Starting State Machine Server - Rel. %s", server.Release)

    if *localOnly {
        logger.Info("Listening on local interface only")
        host = "localhost"
    } else {
        logger.Warn("Listening on all interfaces")
    }
    addr := fmt.Sprintf("%s:%d", host, *port)

    if *redisUrl == "" {
        logger.Warn("in-memory storage configured, all data will NOT survive a server restart")
        store = storage.NewInMemoryStore()
    } else {
        logger.Info("Connecting to Redis server at %s", *redisUrl)
        logger.Info("with timeout: %s, max-retries: %d", *timeout, *maxRetries)
        store = storage.NewRedisStore(*redisUrl, 1, *timeout, *maxRetries)
    }
    server.SetStore(store)

    if *eventsTopic == "" {
        logger.Fatal(fmt.Errorf("no event topic configured, state machines will not " +
            "be able to receive events"))
    }
    logger.Info("Connecting to SQS Topic: %s", *eventsTopic)
    sub = pubsub.NewSqsSubscriber(eventsCh, awsEndpoint)
    if sub == nil {
        panic("Cannot create a valid SQS Subscriber")
    }

    if *dlqTopic != "" {
        logger.Info("Configuring DLQ Topic: %s", *dlqTopic)
        errorsCh = make(chan protos.EventResponse)
        defer close(errorsCh)
        pub = pubsub.NewSqsPublisher(errorsCh, awsEndpoint)
        if pub == nil {
            panic("Cannot create a valid SQS Publisher")
        }
        go pub.Publish(*dlqTopic)
    }
    listener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
        EventsChannel:        eventsCh,
        NotificationsChannel: errorsCh,
        StatemachinesStore:   store,
        // TODO: workers pool not implemented yet.
        ListenersPoolSize: 0,
    })
    go sub.Subscribe(*eventsTopic, nil)

    // This should not be invoked until we have initialized all the services.
    setLogLevel(*debug, *trace)

    logger.Info("Starting Events Listener")
    go listener.ListenForMessages()

    logger.Info("gRPC Server running at tcp://:%d", *grpcPort)
    go startGrpcServer(*grpcPort, eventsCh)

    // TODO: configure & start server using TLS, if configured to do so.
    scheme := "http"
    logger.Info("HTTP Server (REST API) running at %s://%s", scheme, addr)
    srv := server.NewHTTPServer(addr, serverLogLevel)
    logger.Fatal(srv.ListenAndServe())
}

// setLogLevel sets the logging level for all the services' loggers, depending on
// whether the -debug or -trace flag is enabled (if neither, we log at INFO level).
// If both are set, then -trace takes priority.
func setLogLevel(debug bool, trace bool) {
    if debug {
        logger.Info("verbose logging enabled")
        logger.Level = log.DEBUG
        SetLogLevel([]log.Loggable{store, pub, sub, listener}, log.DEBUG)
        serverLogLevel = log.DEBUG
    }

    if trace {
        logger.Warn("trace logging Enabled")
        logger.Level = log.TRACE
        server.EnableTracing()
        SetLogLevel([]log.Loggable{store, sub, listener}, log.TRACE)
        serverLogLevel = log.TRACE
    }
}

// startGrpcServer will start a new gRPC server, bound to
// the local `port` and will send any incoming
// `EventRequest` to the receiving channel.
// This MUST be run as a go-routine, which never returns
func startGrpcServer(port int, events chan<- protos.EventRequest) {
    defer wg.Done()
    l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        panic(err)
    }
    // TODO: should we add a `done` channel?
    grpcServer, err := grpc.NewGrpcServer(&grpc.Config{
        EventsChannel: events,
        Logger:        logger,
        Store:         store,
    })
    err = grpcServer.Serve(l)
    if err != nil {
        panic(err)
    }
}
