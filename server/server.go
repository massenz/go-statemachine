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
	log "github.com/massenz/go-statemachine/logging"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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
	r.HandleFunc("/health", httpsrv.healthHandler).Methods("GET")
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
