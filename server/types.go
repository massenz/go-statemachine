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
	protos "github.com/massenz/statemachine-proto/golang/api"
)

const (
	Authorization   = "Authorization"
	Bearer          = "Bearer"
	ContentType     = "Content-Type"
	ApplicationJson = "application/json"
	AllContent      = "*/*"
	Html            = "text/html"
)

// MessageResponse is returned when a more appropriate response is not available.
type MessageResponse struct {
	Msg   interface{} `json:"message,omitempty"`
	Error string      `json:"error,omitempty"`
}

// StateMachineRequest represents a request for a new FSM to be created, with an optional ID,
// and a reference to a fully qualified Configuration version.
//
// If the ID is not specified, a new UUID will be generated and returned.
type StateMachineRequest struct {
	ID                   string `json:"id"`
	ConfigurationVersion string `json:"configuration_version"`
}

// StateMachineResponse is returned when a new FSM is created, or as a response to a GET request
type StateMachineResponse struct {
	ID           string                     `json:"id"`
	StateMachine *protos.FiniteStateMachine `json:"statemachine"`
}

// EventResponse is returned as a response to a GET Event request
type EventResponse struct {
	ID    string        `json:"id"`
	Event *protos.Event `json:"event"`
}

type OutcomeResponse struct {
	StatusCode  string `json:"status_code"`
	Message     string `json:"message"`
	Destination string `json:"destination"`
}

func MakeOutcomeResponse(outcome *protos.EventOutcome) *OutcomeResponse {
	return &OutcomeResponse{
		StatusCode:  outcome.Code.String(),
		Message:     outcome.Details,
		Destination: outcome.Dest,
	}
}
