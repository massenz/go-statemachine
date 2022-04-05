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
	"github.com/massenz/go-statemachine/api"
	log "github.com/massenz/go-statemachine/logging"
	"sync"
)

func NewInMemoryStore() StoreManager {
	var store = InMemoryStore{
		configurationsStore: make(map[string][]byte),
		machinesStore:       make(map[string][]byte),
		logger:              log.NewLog("memory_store"),
	}
	return &store
}

type InMemoryStore struct {
	mux                 sync.RWMutex
	configurationsStore map[string][]byte
	machinesStore       map[string][]byte
	logger              *log.Log
}

// GetLog allows InMemoryStore to implement the log.Loggable interface
func (csm *InMemoryStore) GetLog() *log.Log {
	return csm.logger
}

func (csm *InMemoryStore) GetConfig(id string) (cfg *api.Configuration, ok bool) {
	csm.logger.Debug("Fetching Configuration [%s]", id)

	csm.mux.RLock()
	defer csm.mux.RUnlock()

	cfgBytes, ok := csm.configurationsStore[id]
	csm.logger.Debug("Found: %q", ok)
	if ok {
		cfg = &api.Configuration{}
		err := cfg.UnmarshalBinary(cfgBytes)
		if err != nil {
			csm.logger.Error(err.Error())
			return nil, false
		}
	}
	return
}

func (csm *InMemoryStore) PutConfig(id string, cfg *api.Configuration) (err error) {
	csm.mux.Lock()
	defer csm.mux.Unlock()
	val, err := cfg.MarshalBinary()
	if err == nil {
		csm.configurationsStore[id] = val
	}
	return err
}

func (csm *InMemoryStore) GetStateMachine(id string) (machine *api.FiniteStateMachine, ok bool) {
	csm.logger.Debug("Fetching StateMachine [%s]", id)
	csm.mux.RLock()
	defer csm.mux.RUnlock()

	machineBytes, ok := csm.machinesStore[id]
	csm.logger.Debug("Found: %q", ok)
	if ok {
		machine = &api.FiniteStateMachine{}
		if err := machine.UnmarshalBinary(machineBytes); err != nil {
			csm.logger.Error(err.Error())
			return nil, false
		}
	}
	return
}

func (csm *InMemoryStore) PutStateMachine(id string, machine *api.FiniteStateMachine) (err error) {
	csm.mux.Lock()
	defer csm.mux.Unlock()

	val, err := machine.MarshalBinary()
	if err == nil {
		csm.machinesStore[id] = val
	}
	return
}
