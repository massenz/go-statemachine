/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package storage

import (
	"fmt"
	log "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"time"
)

func Error(msg string) func(string) error {
	return func(key string) error {
		return fmt.Errorf(msg, key)
	}
}

var (
	IllegalStoreError   = Error("error storing invalid data: %v")
	AlreadyExistsError  = Error("key %s already exists")
	NotFoundError       = Error("key %s not found")
	NotImplementedError = Error("functionality %s has not been implemented yet")
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
