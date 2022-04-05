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
	log "github.com/massenz/go-statemachine/logging"
	"github.com/massenz/go-statemachine/server"
	"github.com/massenz/go-statemachine/storage"
)

const (
	defaultPort  = 8080
	defaultDebug = false
)

func main() {
	var debug = flag.Bool("debug", defaultDebug,
		"If set, URL handlers will emit a trace log for every request; it may impact performance, "+
			"do not use in production or on heavy load systems")
	var localOnly = flag.Bool("local", false,
		"If set, it only listens to incoming requests from the local host")
	var port = flag.Int("port", defaultPort, "Server port")
	var redisUrl = flag.String("redis", "", "URI for the Redis cluster (host:port)")
	var kafkaUrl = flag.String("kafka", "", "URI for the Kafka broker (host:port)")
	var sqsTopic = flag.String("sqs", "", "If defined, it will attempt to connect "+
		"to the given SQS Queue (ignores any value that is passed via the -kafka flag)")
	flag.Parse()

	logger := log.NewLog("statemachine")
	logger.Level = log.INFO

	var host = "0.0.0.0"
	if *localOnly {
		logger.Info("Listening on local interface only")
		host = "localhost"
	} else {
		logger.Warn("Listening on all interfaces")
	}
	addr := fmt.Sprintf("%s:%d", host, *port)
	if *debug {
		logger.Level = log.DEBUG
		logger.Debug("Emitting DEBUG logs")
		server.EnableTracing()
	}

	var store storage.StoreManager
	if *redisUrl == "" {
		logger.Warn("in-memory storage configured, all data will NOT survive a server restart")
		store = storage.NewInMemoryStore()
	} else {
		logger.Info("Connecting to Redis server at %s", *redisUrl)
		store = storage.NewRedisStore(*redisUrl, 1)
	}
	store.GetLog().Level = log.DEBUG
	server.SetStore(store)

	if *kafkaUrl != "" {
		logger.Panic("support for Kafka not implemented")
	} else if *sqsTopic != "" {
		logger.Panic("support for SQS not implemented")
	} else {
		logger.Warn("No event broker configured, state machines will not be " +
			"able to receive events")
	}

	logger.Info("Server started at http://%s", addr)
	srv := server.NewHTTPServer(addr, logger.Level)

	// TODO: configure & start server using TLS, if configured to do so.
	logger.Fatal(srv.ListenAndServe())
}
