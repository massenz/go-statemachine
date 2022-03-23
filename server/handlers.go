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
	"github.com/gorilla/mux"
	"github.com/massenz/go-statemachine/api"
	"net/http"
)

// NOTE: We make the handlers "exportable" so they can be tested, do NOT call directly.

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Standard preamble for all handlers, sets tracing (if enabled) and default content type.
	defer trace(r.RequestURI)()
	defaultContent(w)

	res := MessageResponse{"UP"}
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func CreateConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	defer trace(r.RequestURI)()
	defaultContent(w)

	var config api.Configuration
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if config.Version == "" {
		config.Version = "v1"
	}

	err = config.CheckValid()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = PutConfig(config.GetVersionId(), &config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Location", ConfigurationsEndpoint+"/"+config.GetVersionId())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
	return
}

func GetConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	defer trace(r.RequestURI)()
	defaultContent(w)

	vars := mux.Vars(r)
	if vars == nil {
		logger.Error("Unexpected missing path parameter cfgId in Request URI: %s",
			r.RequestURI)
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}

	cfgId := vars["cfg_id"]
	data, ok := configurationsStore[cfgId]
	if !ok {
		http.Error(w, fmt.Sprintf("Configuration %s does not exist on this server", cfgId),
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
