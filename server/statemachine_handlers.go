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
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/massenz/go-statemachine/storage"
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
	if request.Configuration == "" {
		http.Error(w, "must always specify a fully qualified configuration version",
			http.StatusBadRequest)
		return
	}
	if request.ID == "" {
		request.ID = uuid.New().String()
	} else {
		logger.Debug("checking whether FSM [%s] already exists", request.ID)
		configNameParts := strings.Split(request.Configuration,
			storage.KeyPrefixComponentsSeparator)
		if len(configNameParts) != 2 {
			http.Error(w, fmt.Sprintf("config name is not properly formatted (name:version): %s",
				request.Configuration), http.StatusBadRequest)
			return
		}
		if _, found := storeManager.GetStateMachine(request.ID, configNameParts[0]); found {
			logger.Debug("FSM already exists, returning a Conflict error")
			http.Error(w, storage.AlreadyExistsError(request.ID).Error(), http.StatusConflict)
			return
		}
	}

	logger.Debug("looking up Config [%s]", request.Configuration)
	cfg, ok := storeManager.GetConfig(request.Configuration)
	if !ok {
		http.Error(w, "configuration not found", http.StatusNotFound)
		return
	}
	logger.Debug("found configuration [%s]", cfg)
	logger.Info("Creating a new statemachine [%s] (configuration [%s])",
		request.ID, GetVersionId(cfg))
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

// ModifyStatemachineHandler handles PUT request and can be used to change an
// existing FSM to use a different configuration and, optionally, to change its state.
// Events history CANNOT be altered.
func ModifyStatemachineHandler(w http.ResponseWriter, r *http.Request) {
	defer trace(r.RequestURI)()
	defaultContent(w)

	vars := mux.Vars(r)
	if vars == nil {
		logger.Error("unexpected missing path parameters in Request URI: %s",
			r.RequestURI)
		http.Error(w, UnexpectedError.Error(), http.StatusMethodNotAllowed)
		return
	}
	cfgName := vars["cfg_name"]
	fsmId := vars["sm_id"]

	var request StateMachineChangeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fsm, found := storeManager.GetStateMachine(fsmId, cfgName)
	if !found {
		http.Error(w, storage.NotFoundError(fsmId).Error(), http.StatusNotFound)
		return
	}

	if request.ConfigurationVersion != "" {
		// If the Configuration is specified in the request, we assume
		// that the caller wanted to update it.
		configId := strings.Join([]string{cfgName, request.ConfigurationVersion}, storage.KeyPrefixComponentsSeparator)
		logger.Debug("looking up Config [%s]", configId)
		cfg, ok := storeManager.GetConfig(configId)
		if !ok {
			http.Error(w, "configuration not found", http.StatusNotFound)
			return
		}
		logger.Debug("found configuration [%s]", cfg)
		fsm.ConfigId = GetVersionId(cfg)
	}
	if request.CurrentState != "" && request.CurrentState != fsm.State {
		logger.Debug("changing FSM state from [%s] to [%s]", fsm.State, request.CurrentState)
		fsm.State = request.CurrentState
	}
	err = storeManager.PutStateMachine(fsmId, fsm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(&StateMachineResponse{
		ID:           fsmId,
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
		logger.Error("unexpected missing path parameters in Request URI: %s",
			r.RequestURI)
		http.Error(w, UnexpectedError.Error(), http.StatusMethodNotAllowed)
		return
	}
	cfgName := vars["cfg_name"]
	smId := vars["sm_id"]

	logger.Debug("looking up FSM [%s]", storage.NewKeyForMachine(smId, cfgName))
	stateMachine, ok := storeManager.GetStateMachine(smId, cfgName)
	if !ok {
		http.Error(w, "FSM not found", http.StatusNotFound)
		return
	}
	logger.Debug("found FSM in state '%s'", stateMachine.GetState())

	err := json.NewEncoder(w).Encode(&StateMachineResponse{
		ID:           smId,
		StateMachine: stateMachine,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
