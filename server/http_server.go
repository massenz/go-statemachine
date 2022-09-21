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
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
    "net/http"
    "strings"
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
    r.HandleFunc(ConfigurationsEndpoint, CreateConfigurationHandler).Methods("POST")
    r.HandleFunc(strings.Join([]string{ConfigurationsEndpoint, "{cfg_id}"}, "/"),
        GetConfigurationHandler).Methods("GET")
    r.HandleFunc(StatemachinesEndpoint, CreateStatemachineHandler).Methods("POST")
    r.HandleFunc(strings.Join([]string{StatemachinesEndpoint, "{cfg_name}", "{sm_id}"}, "/"),
        GetStatemachineHandler).Methods("GET")
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
