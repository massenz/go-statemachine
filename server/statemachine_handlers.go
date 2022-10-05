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
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"strings"

	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/statemachine-proto/golang/api"
)

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
		http.Error(w, "configuration not found", http.StatusNotFound)
		return
	}
	logger.Debug("Found configuration %s", cfg)
	if request.ID == "" {
		request.ID = uuid.New().String()
	}
	logger.Info("Creating a new statemachine: %s (configuration: %s)",
		request.ID, request.ConfigurationVersion)

	fsm := &api.FiniteStateMachine{
		ConfigId: GetVersionId(cfg),
		State:    cfg.StartingState,
		History:  make([]*api.Event, 0),
	}
	err = storeManager.PutStateMachine(request.ID, fsm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Location", strings.Join([]string{ApiPrefix, StatemachinesEndpoint, cfg.Name,
		request.ID}, "/"))
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(&StateMachineResponse{
		ID:           request.ID,
		StateMachine: fsm,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func GetStatemachineHandler(w http.ResponseWriter, r *http.Request) {
	defer trace(r.RequestURI)()
	defaultContent(w)

	vars := mux.Vars(r)
	if vars == nil {
		logger.Error("Unexpected missing path parameter smId in Request URI: %s",
			r.RequestURI)
		http.Error(w, UnexpectedError.Error(), http.StatusMethodNotAllowed)
		return
	}

	cfgName := vars["cfg_name"]
	smId := vars["sm_id"]
	logger.Debug("Looking up FSM %s#%s", cfgName, smId)

	stateMachine, ok := storeManager.GetStateMachine(smId, cfgName)
	if !ok {
		http.Error(w, "State Machine not found", http.StatusNotFound)
		return
	}
	logger.Debug("Found FSM: %s", stateMachine.String())

	err := json.NewEncoder(w).Encode(&StateMachineResponse{
		ID:           smId,
		StateMachine: stateMachine,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
