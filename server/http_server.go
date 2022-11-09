/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package server

import (
	"github.com/massenz/go-statemachine/storage"
	log "github.com/massenz/slf4go/logging"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const (
	ApiPrefix              = "/api/v1"
	HealthEndpoint         = "/health"
	ConfigurationsEndpoint = "configurations"
	StatemachinesEndpoint  = "statemachines"
	EventsEndpoint         = "events"
	EventsOutcome          = "outcome"
)

var (
	// Release carries the version of the binary, as set by the build script
	// See: https://blog.alexellis.io/inject-build-time-vars-golang/
	Release string

	shouldTrace  bool
	logger       = log.NewLog("server")
	storeManager storage.StoreManager
)

func trace(endpoint string) func() {
	if !shouldTrace {
		return func() {}
	}
	start := time.Now()
	logger.Trace("Handling: [%s]\n", endpoint)
	return func() { logger.Trace("%s took %s\n", endpoint, time.Since(start)) }
}

func defaultContent(w http.ResponseWriter) {
	w.Header().Add(ContentType, ApplicationJson)
}

func EnableTracing() {
	shouldTrace = true
	logger.Level = log.TRACE
}

func SetLogLevel(level log.LogLevel) {
	logger.Level = level
}

// NewRouter returns a gorilla/mux Router for the server routes; exposed so
// that path params are testable.
func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc(HealthEndpoint, HealthHandler).Methods("GET")

	r.HandleFunc(strings.Join([]string{ApiPrefix, ConfigurationsEndpoint}, "/"),
		CreateConfigurationHandler).Methods("POST")
	r.HandleFunc(strings.Join([]string{ApiPrefix, ConfigurationsEndpoint, "{cfg_id}"}, "/"),
		GetConfigurationHandler).Methods("GET")

	r.HandleFunc(strings.Join([]string{ApiPrefix, StatemachinesEndpoint}, "/"),
		CreateStatemachineHandler).Methods("POST")
	r.HandleFunc(strings.Join([]string{ApiPrefix, StatemachinesEndpoint, "{cfg_name}", "{sm_id}"}, "/"),
		GetStatemachineHandler).Methods("GET")
	r.HandleFunc(strings.Join([]string{ApiPrefix, StatemachinesEndpoint, "{cfg_name}", "{sm_id}"}, "/"),
		ModifyStatemachineHandler).Methods("PUT")

	r.HandleFunc(strings.Join([]string{ApiPrefix, EventsEndpoint, "{cfg_name}", "{evt_id}"}, "/"),
		GetEventHandler).Methods("GET")
	r.HandleFunc(strings.Join([]string{ApiPrefix, EventsEndpoint, EventsOutcome, "{cfg_name}", "{evt_id}"}, "/"),
		GetOutcomeHandler).Methods("GET")

	return r
}

func NewHTTPServer(addr string, logLevel log.LogLevel) *http.Server {
	logger.Level = logLevel
	return &http.Server{
		Addr:    addr,
		Handler: NewRouter(),
	}
}

func SetStore(store storage.StoreManager) {
	storeManager = store
}
