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
    "context"
    "fmt"
    "github.com/go-redis/redis/v8"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "time"
)

var (
    IllegalStoreError   = fmt.Errorf("error storing invalid data")
    ConfigNotFoundError = fmt.Errorf("configuration not found")
    FSMNotFoundError    = fmt.Errorf("statemachine not found")
    NotImplementedError = fmt.Errorf("this functionality has not been implemented yet")
)

type ConfigurationStorageManager interface {
    GetConfig(versionId string) (*protos.Configuration, bool)
    PutConfig(cfg *protos.Configuration) error
}

type FiniteStateMachineStorageManager interface {
    GetStateMachine(id string, cfg string) (*protos.FiniteStateMachine, bool)
    PutStateMachine(id string, fsm *protos.FiniteStateMachine) error
    GetAllInState(cfg string, state string) []*protos.FiniteStateMachine
}

type EventStorageManager interface {
    GetEvent(id string, cfg string) (*protos.Event, bool)
    PutEvent(event *protos.Event, cfg string, ttl time.Duration) error

    // AddEventOutcome adds the outcome of an event to the storage, given the `eventId` and the
    // "type" (`Configuration.Name`) of the FSM that received the event.
    //
    // Optionally, it will remove the outcome after a given `ttl` (time-to-live); use
    // `NeverExpire` to keep the outcome forever.
    AddEventOutcome(eventId string, cfgName string, response *protos.EventOutcome,
        ttl time.Duration) error

    // GetOutcomeForEvent returns the outcome of an event, given the `eventId` and the "type" of the
    // FSM that received the event.
    GetOutcomeForEvent(eventId string, cfgName string) (*protos.EventOutcome, bool)
}

type StoreManager interface {
    log.Loggable
    ConfigurationStorageManager
    FiniteStateMachineStorageManager
    EventStorageManager
    SetTimeout(duration time.Duration)
    GetTimeout() time.Duration
    Health() error
}

type RedisClient interface {
    Get(ctx context.Context, id string) *redis.StringCmd
    Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
    Ping(ctx context.Context) *redis.StatusCmd
}
