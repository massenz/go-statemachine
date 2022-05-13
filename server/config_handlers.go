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
    "github.com/gorilla/mux"
    "github.com/massenz/go-statemachine/api"
    pj "google.golang.org/protobuf/encoding/protojson"
    "net/http"
)

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
    logger.Debug("Creating new configuration with Version ID: %s", config.GetVersionId())

    // TODO: Check this configuration does not already exist.

    err = config.CheckValid()
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    logger.Debug("Configuration is valid (starting state: %s)", config.StartingState)

    err = storeManager.PutConfig(config.GetVersionId(), &config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    logger.Debug("Configuration stored with ID: %s", config.GetVersionId())

    w.Header().Add("Location", ConfigurationsEndpoint+"/"+config.GetVersionId())
    w.WriteHeader(http.StatusCreated)
    err = json.NewEncoder(w).Encode(&config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    return
}

func GetConfigurationHandler(w http.ResponseWriter, r *http.Request) {
    defer trace(r.RequestURI)()
    defaultContent(w)

    vars := mux.Vars(r)
    if vars == nil {
        logger.Error("Unexpected missing path parameter in Request URI: %s",
            r.RequestURI)
        http.Error(w, api.UnexpectedError.Error(), http.StatusInternalServerError)
        return
    }

    cfgId := vars["cfg_id"]
    config, ok := storeManager.GetConfig(cfgId)
    if !ok {
        http.Error(w, fmt.Sprintf("Configuration [%s] not found", cfgId), http.StatusNotFound)
        return
    }
    resp, err := pj.Marshal(config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Write(resp)
    return
}
