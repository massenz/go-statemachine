/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	g "google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	"github.com/massenz/go-statemachine/pubsub"
	"github.com/massenz/go-statemachine/storage"
)

func SetLogLevel(services []log.Loggable, level log.LogLevel) {
	for _, s := range services {
		if s != nil {
			s.SetLogLevel(level)
		}
	}
}

var (
	logger = log.NewLog("sm-server")

	listener *pubsub.EventsListener
	pub      *pubsub.SqsPublisher = nil
	sub      *pubsub.SqsSubscriber
	store    storage.StoreManager
	wg       sync.WaitGroup

	// notificationsCh is the channel over which we send error notifications
	// to publish on the appropriate queue.
	// The Listener will produce error notifications, which will be consumed
	// by the PubSub Publisher (if configured) which in turn will produce to
	// the -notifications topic.
	//
	// Not configured by default, it is only used if a -notifications queue
	// is defined.
	notificationsCh chan protos.EventResponse = nil

	// eventsCh is the channel over which the Listener receive Events to process.
	// Both the gRPC server and the PubSub Subscriber (if configured) will produce
	// events for this channel.
	//
	// Currently, this is a blocking channel (capacity for one item), but once we
	// parallelize events processing we can make it deeper.
	eventsCh = make(chan protos.EventRequest)
)

func main() {

	var awsEndpoint = flag.String("endpoint-url", "",
		"HTTP URL for AWS SQS to connect to; usually best left undefined, "+
			"unless required for local testing purposes (LocalStack uses http://localhost:4566)")
	var cluster = flag.Bool("cluster", false,
		"If set, connects to Redis with cluster-mode enabled")
	var debug = flag.Bool("debug", false,
		"Verbose logs; better to avoid on Production services")
	var eventsTopic = flag.String("events", "", "Topic name to receive events from")
	var grpcPort = flag.Int("grpc-port", 7398, "The port for the gRPC server")
	var noTls = flag.Bool("insecure", false, "If set, TLS will be disabled (NOT recommended)")
	var maxRetries = flag.Int("max-retries", storage.DefaultMaxRetries,
		"Max number of attempts for a recoverable error to be retried against the Redis cluster")
	var notificationsTopic = flag.String("notifications", "",
		"(optional) The name of the topic to publish events' outcomes to; if not "+
			"specified, no outcomes will be published")
	var redisUrl = flag.String("redis", "", "For single node Redis instances: host:port "+
		"for the Redis instance. For redis clusters: a comma-separated list of redis nodes. "+
		"If using an ElastiCache Redis cluster with cluster mode enabled, this can also be the configuration endpoint.")
	var timeout = flag.Duration("timeout", storage.DefaultTimeout,
		"Timeout for Redis (as a Duration string, e.g. 1s, 20ms, etc.)")
	var trace = flag.Bool("trace", false,
		"Extremely verbose logs for every API request and Pub/Sub event; it may impact"+
			" performance, do not use in production or on heavily loaded systems (will override the -debug option)")
	flag.Parse()

	logger.Info("starting State Machine Server - Rel. %s", api.Release)

	if *redisUrl == "" {
		logger.Fatal(errors.New("in-memory store deprecated, a Redis server must be configured"))
	} else {
		logger.Info("connecting to Redis server at %s", *redisUrl)
		logger.Info("with timeout: %s, max-retries: %d", *timeout, *maxRetries)
		store = storage.NewRedisStore(*redisUrl, *cluster, 1, *timeout, *maxRetries)
	}
	done := make(chan interface{})
	if *eventsTopic != "" {
		logger.Info("connecting to SQS Topic: %s", *eventsTopic)
		sub = pubsub.NewSqsSubscriber(eventsCh, awsEndpoint)
		if sub == nil {
			logger.Fatal(errors.New("cannot create a valid SQS Subscriber"))
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("subscribing to events on topic [%s]", *eventsTopic)
			sub.Subscribe(*eventsTopic, done)
		}()
	}

	if *notificationsTopic != "" {
		logger.Info("sending errors to DLQ topic [%s]", *notificationsTopic)
		notificationsCh = make(chan protos.EventResponse)
		defer close(notificationsCh)
		pub = pubsub.NewSqsPublisher(notificationsCh, awsEndpoint)
		if pub == nil {
			logger.Fatal(errors.New("cannot create a valid SQS Publisher"))
		}
		go pub.Publish(*notificationsTopic)
	}

	listener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
		EventsChannel:        eventsCh,
		NotificationsChannel: notificationsCh,
		StatemachinesStore:   store,
		// TODO: workers pool not implemented yet.
		ListenersPoolSize: 0,
	})
	logger.Info("starting events listener")
	wg.Add(1)
	go func() {
		defer wg.Done()
		listener.ListenForMessages()
	}()

	logger.Info("gRPC server running at tcp://:%d", *grpcPort)
	svr := startGrpcServer(*grpcPort, *noTls, eventsCh)

	// This should not be invoked until we have initialized all the services.
	setLogLevel(*debug, *trace)
	logger.Info("statemachine server ready for processing events...")
	RunUntilStopped(done, svr)
	logger.Info("...done. Goodbye.")
}

func RunUntilStopped(done chan interface{}, svr *g.Server) {
	// Trap Ctrl-C and SIGTERM (Docker/Kubernetes) to shutdown gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received.
	_ = <-c
	logger.Info("shutting down services...")
	close(done)
	close(eventsCh)
	svr.GracefulStop()
	logger.Info("waiting for services to exit...")
	wg.Wait()
}

// setLogLevel sets the logging level for all the services' loggers, depending on
// whether the -debug or -trace flag is enabled (if neither, we log at INFO level).
// If both are set, then -trace takes priority.
func setLogLevel(debug bool, trace bool) {
	var logLevel log.LogLevel = log.INFO
	if debug && !trace {
		logger.Info("verbose logging enabled")
		logLevel = log.DEBUG
	} else if trace {
		logger.Info("trace logging enabled")
		logLevel = log.TRACE
	}
	logger.Level = logLevel
	SetLogLevel([]log.Loggable{store, pub, sub, listener}, logLevel)
}

// startGrpcServer will start a new gRPC server, bound to
// the local `port` and will send any incoming
// `EventRequest` to the receiving channel.
// This MUST be run as a go-routine, which never returns
func startGrpcServer(port int, disableTls bool, events chan<- protos.EventRequest) *g.Server {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	// TODO: should we add a `done` channel?
	grpcServer, err := grpc.NewGrpcServer(&grpc.Config{
		EventsChannel: events,
		Logger:        logger,
		Store:         store,
		TlsEnabled:    !disableTls,
	})
	if err != nil {
		logger.Fatal(err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = grpcServer.Serve(l)
		if err != nil {
			logger.Fatal(err)
		}
		logger.Info("gRPC server exited")
	}()
	return grpcServer
}
