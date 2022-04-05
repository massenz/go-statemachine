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

package storage

import (
	"fmt"
	"github.com/massenz/go-statemachine/api"
	log "github.com/massenz/go-statemachine/logging"
)

var IllegalStoreError = fmt.Errorf("error storing invalid data")

type ConfigurationStorageManager interface {
	GetConfig(id string) (cfg *api.Configuration, ok bool)
	PutConfig(id string, cfg *api.Configuration) (err error)
}

type FiniteStateMachineStorageManager interface {
	GetStateMachine(id string) (cfg *api.FiniteStateMachine, ok bool)
	PutStateMachine(id string, cfg *api.FiniteStateMachine) (err error)
}

type StoreManager interface {
	log.Loggable
	ConfigurationStorageManager
	FiniteStateMachineStorageManager
}
