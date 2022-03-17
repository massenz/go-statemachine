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

package server

import (
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/massenz/go-statemachine/api"
	log "github.com/massenz/go-statemachine/logging"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const (
	Api                    = "/api/v1"
	HealthEndpoint         = "/health"
	ConfigurationsEndpoint = Api + "/configurations"
	StatemachinesEndpoint  = Api + "/statemachines"
)

func (s *httpServer) trace(endpoint string) func() {
	if !s.shouldTrace {
		return func() {}
	}
	start := time.Now()
	s.log.Trace("Handling: [%s]\n", endpoint)
	return func() { s.log.Trace("%s took %s\n", endpoint, time.Since(start)) }
}

func defaultContent(w http.ResponseWriter) {
	w.Header().Add(ContentType, ApplicationJson)
}

func newHTTPServer(log *log.Log) *httpServer {
	return &httpServer{
		log:         log,
		shouldTrace: traceEnabled,
	}
}

var traceEnabled bool

func EnableTracing(enable bool) {
	traceEnabled = enable
}

func NewHTTPServer(addr string, logger *log.Log) *http.Server {
	var httpsrv *httpServer
	if logger == nil {
		httpsrv = newHTTPServer(log.NewLog())
	} else {
		httpsrv = newHTTPServer(logger)
	}
	r := mux.NewRouter()
	r.HandleFunc(HealthEndpoint, httpsrv.healthHandler).Methods("GET")
	r.HandleFunc(ConfigurationsEndpoint, httpsrv.createConfigurationHandler).
		Methods("POST")
	r.HandleFunc(ConfigurationsEndpoint+"/{cfg_id}", httpsrv.getConfigurationHandler).
		Methods("GET")
	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

func (s *httpServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Standard preamble for all handlers, sets tracing (if enabled) and default content type.
	defer s.trace(r.RequestURI)()
	defaultContent(w)

	res := HealthResponse{"UP"}
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// FIXME: This is temporary until we implement a real Storage module.
// We store the serialized PB (instead of *Configuration, e.g.) so that the
// behavior mirrors what will be eventually implemented in Redis.
var configurationsStore = make(map[string][]byte)
var machinesStore = make(map[string][]byte)

func (s *httpServer) createConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	defer s.trace(r.RequestURI)()
	defaultContent(w)

	var config api.Configuration
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if config.Name == "" {
		http.Error(w, api.MissingNameConfigurationError.Error(), http.StatusBadRequest)
		return
	}
	if config.Version == "" {
		config.Version = "v1"
	}
	// TODO: add a validation function to check for well-formed Configuration
	out, err := proto.Marshal(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	configurationsStore[config.GetVersionId()] = out

	w.Header().Add("Location", ConfigurationsEndpoint+"/"+config.GetVersionId())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
	return
}

func (s *httpServer) getConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	defer s.trace(r.RequestURI)()
	defaultContent(w)

	vars := mux.Vars(r)
	cfg_id := vars["cfg_id"]
	data, ok := configurationsStore[cfg_id]
	if !ok {
		http.Error(w, fmt.Sprintf("Configuration %s does not exist on this server", cfg_id),
			http.StatusNotFound)
		return
	}
	var config api.Configuration
	err := proto.Unmarshal(data, &config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(config)
	return
}
