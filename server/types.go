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

import "github.com/massenz/go-statemachine/api"

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
    ID           string                  `json:"id"`
    StateMachine *api.FiniteStateMachine `json:"statemachine"`
}
