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
	"strconv"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/massenz/go-statemachine/pkg/api"
	"github.com/massenz/go-statemachine/pkg/grpc"
	"github.com/massenz/go-statemachine/pkg/pubsub"
	"github.com/massenz/go-statemachine/pkg/storage"
	g "google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

var (
	logger = zlog.With().Str("logger", "fsmsrv").Logger()

	listener *pubsub.EventsListener
	pub      *pubsub.SqsPublisher
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
	// Both the gRPC Server and the PubSub Subscriber (if configured) will produce
	// events for this channel.
	//
	// Currently, this is a blocking channel (capacity for one item), but once we
	// parallelize events processing we can make it deeper.
	eventsCh = make(chan protos.EventRequest)
)

func main() {
	// Global zerolog configuration.
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zlog.Logger = zlog.Output(os.Stderr)

	var awsEndpoint = flag.String("endpoint-url", "",
		"HTTP URL for AWS SQS to connect to; usually best left undefined, "+
			"unless required for local testing purposes (LocalStack uses http://localhost:4566)")
	var cluster = flag.Bool("cluster", false,
		"If set, connects to Redis with cluster-mode enabled")
	var debug = flag.Bool("debug", false,
		"Verbose logs; better to avoid on Production services")
	var eventsTopic = flag.String("events", "", "Topic name to receive events from")
	var grpcPort = flag.Int("grpc-port", 7398, "The port for the gRPC Server")
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

	logger.Info().Str("release", api.Release).Msg("starting State Machine Server")

	if *redisUrl == "" {
		logger.Fatal().Err(errors.New("in-memory store deprecated, a Redis server must be configured")).Msg("fatal configuration error")
	} else {
		logger.Info().
			Str("redis_addr", *redisUrl).
			Str("redis_cluster", strconv.FormatBool(*cluster)).
			Str("redis_timeout", timeout.String()).
			Str("redis_max_retries", strconv.Itoa(*maxRetries)).
			Msg("connecting to Redis server")
		store = storage.NewRedisStore(*redisUrl, *cluster, 1, *timeout, *maxRetries)
	}
	done := make(chan interface{})
	if *eventsTopic != "" {
		logger.Info().
			Str("sqs_topic", *eventsTopic).
			Str("sqs_endpoint", *awsEndpoint).
			Msg("connecting to SQS topic for incoming events")
		sub = pubsub.NewSqsSubscriber(eventsCh, awsEndpoint)
		if sub == nil {
			logger.Fatal().Err(errors.New("cannot create a valid SQS Subscriber")).Msg("fatal error creating SQS subscriber")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info().Msgf("subscribing to events on topic [%s]", *eventsTopic)
			sub.Subscribe(*eventsTopic, done)
		}()
	}

	if *notificationsTopic != "" {
		logger.Info().
			Str("sqs_dlq_topic", *notificationsTopic).
			Str("sqs_endpoint", *awsEndpoint).
			Msg("sending error notifications to DLQ topic")
		notificationsCh = make(chan protos.EventResponse)
		defer close(notificationsCh)
		pub = pubsub.NewSqsPublisher(notificationsCh, awsEndpoint)
		if pub == nil {
			logger.Fatal().Err(errors.New("cannot create a valid SQS Publisher")).Msg("fatal error creating SQS publisher")
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
	logger.Info().Msg("starting events listener")
	wg.Add(1)
	go func() {
		defer wg.Done()
		listener.ListenForMessages()
	}()

	logger.Info().Str("grpc_port", strconv.Itoa(*grpcPort)).Msg("gRPC server starting")
	svr := startGrpcServer(*grpcPort, *noTls, eventsCh)

	// This should not be invoked until we have initialized all the services.
	setLogLevel(*debug, *trace)
	logger.Info().Msg("statemachine server ready for processing events...")
	RunUntilStopped(done, svr)
	logger.Info().Msg("...done. Goodbye.")
}

func RunUntilStopped(done chan interface{}, svr *g.Server) {
	// Trap Ctrl-C and SIGTERM (Docker/Kubernetes) to shutdown gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received.
	_ = <-c
	logger.Info().Msg("shutting down services...")
	close(done)
	close(eventsCh)
	svr.GracefulStop()
	logger.Info().Msg("waiting for services to exit...")
	wg.Wait()
}

// setLogLevel sets the global logging level depending on -debug / -trace.
// If both are set, then -trace takes priority.
func setLogLevel(debug bool, trace bool) {
	level := zerolog.InfoLevel
	if debug && !trace {
		logger.Info().Msg("verbose logging enabled")
		level = zerolog.DebugLevel
	} else if trace {
		logger.Info().Msg("trace logging enabled")
		level = zerolog.TraceLevel
	}
	zerolog.SetGlobalLevel(level)
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
		logger.Fatal().Err(err).Msg("failed to create gRPC server")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
			err = grpcServer.Serve(l)
			if err != nil {
				logger.Fatal().Err(err).Msg("gRPC server exited with error")
			}
			logger.Info().Msg("gRPC Server exited")
	}()
	return grpcServer
}
