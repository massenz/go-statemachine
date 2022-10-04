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
    "github.com/golang/protobuf/proto"
    "github.com/massenz/go-statemachine/api"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "strings"
    "sync"
    "time"
)

func NewInMemoryStore() StoreManager {
    return &InMemoryStore{
        backingStore: make(map[string][]byte),
        logger:       log.NewLog("memory_store"),
    }
}

type InMemoryStore struct {
    logger       *log.Log
    mux          sync.RWMutex
    backingStore map[string][]byte
}

func (csm *InMemoryStore) get(key string, value proto.Message) bool {
    csm.mux.RLock()
    defer csm.mux.RUnlock()

    bytes, ok := csm.backingStore[key]
    csm.logger.Trace("key %s - Found: %t", key, ok)
    if ok {
        err := proto.Unmarshal(bytes, value)
        if err != nil {
            csm.logger.Error(err.Error())
            return false
        }
    }
    return ok
}

func (csm *InMemoryStore) put(key string, value proto.Message) error {
    csm.mux.Lock()
    defer csm.mux.Unlock()

    val, err := proto.Marshal(value)
    if err == nil {
        csm.logger.Trace("Storing key %s [%T]", key, value)
        csm.backingStore[key] = val
    }
    return err
}

func (csm *InMemoryStore) GetAllInState(cfg string, state string) []*protos.FiniteStateMachine {
    // TODO [#33] Ability to query for all machines in a given state
    csm.logger.Error("Not implemented")
    return nil
}

func (csm *InMemoryStore) GetEvent(id string, cfg string) (*protos.Event, bool) {
    key := NewKeyForEvent(id, cfg)
    event := &protos.Event{}
    return event, csm.get(key, event)
}

func (csm *InMemoryStore) PutEvent(event *protos.Event, cfg string, ttl time.Duration) error {
    key := NewKeyForEvent(event.EventId, cfg)
    return csm.put(key, event)
}

func (csm *InMemoryStore) AddEventOutcome(id string, cfg string, response *protos.EventOutcome, ttl time.Duration) error {
    key := NewKeyForOutcome(id, cfg)
    return csm.put(key, response)
}

func (csm *InMemoryStore) GetOutcomeForEvent(id string, cfg string) (*protos.EventOutcome, bool) {
    key := NewKeyForOutcome(id, cfg)
    var outcome protos.EventOutcome
    return &outcome, csm.get(key, &outcome)
}

func (csm *InMemoryStore) GetConfig(id string) (cfg *protos.Configuration, ok bool) {
    key := NewKeyForConfig(id)
    csm.logger.Debug("Fetching Configuration [%s]", key)
    cfg = &protos.Configuration{}
    return cfg, csm.get(key, cfg)
}

func (csm *InMemoryStore) PutConfig(cfg *protos.Configuration) error {
    key := NewKeyForConfig(api.GetVersionId(cfg))
    csm.logger.Debug("Storing Configuration [%s] with key: %s", api.GetVersionId(cfg), key)
    return csm.put(key, cfg)
}

func (csm *InMemoryStore) GetStateMachine(id string, cfg string) (*protos.FiniteStateMachine, bool) {
    csm.logger.Debug("Fetching StateMachine [%s#%s]", cfg, id)
    key := NewKeyForMachine(id, cfg)
    machine := protos.FiniteStateMachine{}
    if csm.get(key, &machine) {
        csm.logger.Debug("Found StateMachine [%s#%s]: %s", cfg, id, machine.State)
        return &machine, true
    }
    return nil, false
}

func (csm *InMemoryStore) PutStateMachine(id string, machine *protos.FiniteStateMachine) error {
    if machine == nil {
        return IllegalStoreError
    }
    key := NewKeyForMachine(id, strings.Split(machine.ConfigId, api.ConfigurationVersionSeparator)[0])
    csm.logger.Debug("Storing StateMachine [%s] with key: %s", id, key)
    return csm.put(key, machine)
}

func (csm *InMemoryStore) SetLogLevel(level log.LogLevel) {
    csm.logger.Level = level
}

// SetTimeout does not really make sense for an in-memory store, so this is a no-op
func (csm *InMemoryStore) SetTimeout(_ time.Duration) {
    // do nothing
}

// GetTimeout does not really make sense for an in-memory store,
// so this just returns a NeverExpire constant.
func (csm *InMemoryStore) GetTimeout() time.Duration {
    return NeverExpire
}

func (csm *InMemoryStore) Health() error {
    return nil
}
