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

var (
	shouldTrace bool
	logger      = log.NewLog("server")
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

func NewHTTPServer(addr string, logLevel log.LogLevel) *http.Server {
	logger.Level = logLevel

	r := mux.NewRouter()
	r.HandleFunc(HealthEndpoint, HealthHandler).Methods("GET")
	r.HandleFunc(ConfigurationsEndpoint, CreateConfigurationHandler).Methods("POST")
	r.HandleFunc(ConfigurationsEndpoint+"/{cfg_id}", GetConfigurationHandler).Methods("GET")
	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

// FIXME: This is temporary until we implement a real Storage module.
// We store the serialized PB (instead of *Configuration, e.g.) so that the
// behavior mirrors what will be eventually implemented in Redis.
// FIXME: AND, even more obviously, these should be protected by Mutexes,
// but that's largely irrelevant as they'll soon be gone (hopefully)
var configurationsStore = make(map[string][]byte)
var machinesStore = make(map[string][]byte)

func GetConfig(id string) (cfg *api.Configuration, ok bool) {
	cfgBytes, ok := configurationsStore[id]
	if ok {
		cfg = &api.Configuration{}
		err := proto.Unmarshal(cfgBytes, cfg)
		if err != nil {
			return nil, false
		}
	}
	return
}
