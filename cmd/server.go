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
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/go-statemachine/server"
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
)

func SetLogLevel(services []log.Loggable, level log.LogLevel) {
    for _, s := range services {
        if s != nil {
            s.SetLogLevel(level)
        }
    }
}

func main() {
    var debug = flag.Bool("debug", false,
        "Verbose logs; better to avoid on Production services")
    var trace = flag.Bool("trace", false,
        "Enables trace logs for every API request and Pub/Sub event; it may impact performance, "+
            "do not use in production or on heavily loaded systems ("+
            "will override the -debug option)")
    var localOnly = flag.Bool("local", false,
        "If set, it only listens to incoming requests from the local host")
    var port = flag.Int("port", 4567, "Server port")
    var redisUrl = flag.String("redis", "", "URI for the Redis cluster (host:port)")
    var awsEndpoint = flag.String("endpoint-url", "",
        "HTTP URL for AWS SQS to connect to; usually best left undefined, "+
            "unless required for local testing purposes (LocalStack uses http://localhost:4566)")
    var eventsTopic = flag.String("events", "", "If defined, it will attempt to connect "+
        "to the given SQS Queue (ignores any value that is passed via the -kafka flag)")
    var dlqTopic = flag.String("errors", "",
        "The name of the Dead-Letter Queue ("+"DLQ) in SQS to post errors to; if not "+
            "specified, the DLQ will not be used")
    flag.Parse()

    logger := log.NewLog("statemachine")

    var host = "0.0.0.0"
    if *localOnly {
        logger.Info("Listening on local interface only")
        host = "localhost"
    } else {
        logger.Warn("Listening on all interfaces")
    }
    addr := fmt.Sprintf("%s:%d", host, *port)

    var store storage.StoreManager
    if *redisUrl == "" {
        logger.Warn("in-memory storage configured, all data will NOT survive a server restart")
        store = storage.NewInMemoryStore()
    } else {
        logger.Info("Connecting to Redis server at %s", *redisUrl)
        store = storage.NewRedisStore(*redisUrl, 1)
    }
    server.SetStore(store)

    var sub *pubsub.SqsSubscriber
    var pub *pubsub.SqsPublisher = nil

    // TODO: for now blocking channels; we will need to confirm
    //  whether we can support a fully concurrent system with a
    //  buffered channel
    var eventsCh = make(chan pubsub.EventMessage)
    defer close(eventsCh)

    var errorsCh chan pubsub.EventErrorMessage = nil

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
        errorsCh = make(chan pubsub.EventErrorMessage)
        defer close(errorsCh)
        pub = pubsub.NewSqsPublisher(errorsCh, awsEndpoint)
        if sub == nil {
            panic("Cannot create a valid SQS Publisher")
        }
        go pub.Publish(*dlqTopic)
    }
    listener := pubsub.NewEventsListener(&pubsub.ListenerOptions{
        EventsChannel:        eventsCh,
        NotificationsChannel: errorsCh,
        StatemachinesStore:   store,
        // TODO: workers pool not implemented yet.
        ListenersPoolSize: 0,
    })

    var serverLogLevel log.LogLevel = log.INFO
    if *debug {
        logger.Info("verbose logging enabled")
        logger.Level = log.DEBUG
        SetLogLevel([]log.Loggable{store, pub, sub, listener}, log.DEBUG)
        serverLogLevel = log.DEBUG
    }

    if *trace {
        logger.Warn("trace logging Enabled")
        logger.Level = log.TRACE
        server.EnableTracing()
        SetLogLevel([]log.Loggable{store, sub, listener}, log.TRACE)
        serverLogLevel = log.TRACE
    }

    logger.Info("Starting Subscriber, Publisher and Listener goroutines")
    go listener.ListenForMessages()
    go sub.Subscribe(*eventsTopic, nil)

    // TODO: configure & start server using TLS, if configured to do so.
    logger.Info("Server running at http://%s", addr)
    srv := server.NewHTTPServer(addr, serverLogLevel)
    logger.Fatal(srv.ListenAndServe())
}
