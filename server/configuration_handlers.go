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
	"github.com/gorilla/mux"
	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/statemachine-proto/golang/api"
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
	logger.Debug("Creating new configuration with Version ID: %s", GetVersionId(&config))

	// TODO: Check this configuration does not already exist.

	err = CheckValid(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logger.Debug("Configuration is valid (starting state: %s)", config.StartingState)

	err = storeManager.PutConfig(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Debug("Configuration stored: %s", GetVersionId(&config))

	w.Header().Add("Location", ConfigurationsEndpoint+"/"+GetVersionId(&config))
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
		http.Error(w, UnexpectedError.Error(), http.StatusInternalServerError)
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
