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
    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "github.com/massenz/go-statemachine/api"
    pj "google.golang.org/protobuf/encoding/protojson"
    "net/http"
)

// StateMachineRequest represents a request for a new FSM to be created, with an optional ID,
// and a reference to a fully qualified Configuration version
type StateMachineRequest struct {
    ID                   string `json:"statemachine_id"`
    ConfigurationVersion string `json:"configuration_version"`
}

// StateMachineResponse is returned when a new FSM is created
type StateMachineResponse struct {
    ID                   string `json:"statemachine_id"`
    ConfigurationVersion string `json:"configuration_version"`
    State                string `json:"initial_state"`
}

func CreateStatemachineHandler(w http.ResponseWriter, r *http.Request) {
    defer trace(r.RequestURI)()
    defaultContent(w)

    var request StateMachineRequest
    err := json.NewDecoder(r.Body).Decode(&request)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if request.ConfigurationVersion == "" {
        http.Error(w, "Must always specify a fully qualified configuration version", http.StatusBadRequest)
        return
    }
    cfg, ok := storeManager.GetConfig(request.ConfigurationVersion)
    if !ok {
        http.Error(w, fmt.Sprintf("configuration %s not found", request.ConfigurationVersion),
            http.StatusNotAcceptable)
        return
    }
    logger.Debug("Found configuration %s", cfg)
    if request.ID == "" {
        request.ID = uuid.New().String()
    }
    logger.Info("Creating a new statemachine: %s (configuration: %s)",
        request.ID, request.ConfigurationVersion)

    fsm := &api.FiniteStateMachine{
        ConfigId: cfg.GetVersionId(),
        State:    cfg.StartingState,
        History:  make([]string, 0),
    }
    err = storeManager.PutStateMachine(request.ID, fsm)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Add("Location", StatemachinesEndpoint+"/"+fsm.ConfigId)
    w.WriteHeader(http.StatusCreated)
    err = json.NewEncoder(w).Encode(&StateMachineResponse{
        ID:                   request.ID,
        ConfigurationVersion: fsm.ConfigId,
        State:                fsm.State,
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    return
}

func GetStatemachineHandler(w http.ResponseWriter, r *http.Request) {
    defer trace(r.RequestURI)()
    defaultContent(w)

    vars := mux.Vars(r)
    if vars == nil {
        logger.Error("Unexpected missing path parameter smId in Request URI: %s",
            r.RequestURI)
        http.Error(w, api.UnexpectedError.Error(), http.StatusMethodNotAllowed)
        return
    }

    smId := vars["statemachine_id"]
    stateMachine, ok := storeManager.GetStateMachine(smId)
    if !ok {
        http.Error(w, fmt.Sprintf("State Machine [%s] not found", smId), http.StatusNotFound)
        return
    }
    resp, err := pj.Marshal(stateMachine)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Write(resp)
    return
}
