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
	"net/http"
)

// NOTE: We make the handlers "exportable" so they can be tested, do NOT call directly.

type HealthResponse struct {
	Status  string `json:"status"`
	Release string `json:"release"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Standard preamble for all handlers, sets tracing (if enabled) and default content type.
	defer trace(r.RequestURI)()
	defaultContent(w)

	var response MessageResponse
	res := HealthResponse{
		Status:  "OK",
		Release: Release,
	}
	var err error
	if storeManager == nil {
		err = fmt.Errorf("store manager is not initialized")
	} else {
		err = storeManager.Health()
	}
	if err != nil {
		logger.Error("Health check failed: %s", err)
		res.Status = "ERROR"
		response = MessageResponse{
			Msg:   res,
			Error: fmt.Sprintf("error connecting to storage: %s", err),
		}
	} else {
		response = MessageResponse{
			Msg: res,
		}
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
