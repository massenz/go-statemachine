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

package api

import (
	"fmt"
	"github.com/google/uuid"
	log "github.com/massenz/go-statemachine/logging"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

var MalformedConfigurationError = fmt.Errorf("this configuration cannot be parsed")
var MissingNameConfigurationError = fmt.Errorf("configuration must always specify a name (" +
	"and optionally a version)")
var MissingStatesConfigurationError = fmt.Errorf(
	"configuration must always specify at least one state")
var MismatchStartingstateConfigurationError = fmt.Errorf(
	"the StartingState must be one of the possible FSM states")
var UnreachableStateConfigurationError = "state %s is not used in any of the transitions"

var UnexpectedTransitionError = fmt.Errorf("unexpected event transition")
var UnexpectedEventError = fmt.Errorf("the event was malformed")
var NotImplementedError = fmt.Errorf("not implemented")

// Logger is made accessible so that its `Level` can be changed
// or can be sent to a `NullLog` during testing.
var Logger = log.NewLog("fsm")

// eventsCache is a local cache to store events while this server is running
// TODO: implement a side-load for misses that are fetched from a backing store.
// FIXME: add a Mutex to protect this cache
var eventsCache = make(map[string]*Event)

func GetEvent(eventId string) *Event {
	return eventsCache[eventId]
}

func PutEvent(evt *Event) {
	eventsCache[evt.EventId] = evt
}

// ConfiguredStateMachine is the internal representation of an FSM, which
// carries within itself the configuration for ease of use.
type ConfiguredStateMachine struct {
	Config *Configuration
	FSM    *FiniteStateMachine
}

func NewStateMachine(configuration *Configuration) (*ConfiguredStateMachine, error) {
	if configuration.Name == "" {
		Logger.Error("Missing configuration name")
		return nil, MalformedConfigurationError
	}
	if configuration.Version == "" {
		configuration.Version = "v1"
	}
	return &ConfiguredStateMachine{
		FSM: &FiniteStateMachine{
			ConfigId: configuration.Name + ":" + configuration.Version,
			State:    configuration.StartingState,
			//History:  make([]string, 0),
		},
		Config: configuration,
	}, nil
}

// SendEvent registers the event with the FSM and effects the transition, if valid.
// It also creates a new Event, and stores in the provided cache.
func (x *ConfiguredStateMachine) SendEvent(evt string) error {
	for _, t := range x.Config.Transitions {
		if t.From == x.FSM.State && t.Event == evt {
			event := NewEvent(evt)
			event.Transition.From = x.FSM.State
			x.FSM.State = t.To
			event.Transition.To = x.FSM.State
			x.FSM.History = append(x.FSM.History, event.EventId)
			PutEvent(event)
			return nil
		}
	}
	return UnexpectedTransitionError
}

func NewEvent(evt string) *Event {
	return &Event{
		EventId:    uuid.New().String(),
		Timestamp:  tspb.Now(),
		Transition: &Transition{Event: evt},
	}
}

func (x *ConfiguredStateMachine) Reset() {
	x.FSM.State = x.Config.StartingState
	x.FSM.History = nil
}

func (x *Configuration) GetVersionId() string {
	return x.Name + ":" + x.Version
}

// HasState will check whether a given state is either origin or destination for the Transition
func (x *Transition) HasState(state string) bool {
	return state == x.From || state == x.To
}

// HasState checks that `state` is one of the Configuration's `States`
func (x *Configuration) HasState(state string) bool {
	for _, s := range x.States {
		if s == state {
			return true
		}
	}
	return false
}

// CheckValid checks that the Configuration is valid and that the current FSM's `state` is one of
// the allowed states in the Configuration.
//
// We also check that the reported FSM's ConfigId, matches the Configuration's name, version.
func (x *ConfiguredStateMachine) CheckValid() bool {
	return x.Config.CheckValid() == nil && x.Config.HasState(x.FSM.State) &&
		x.FSM.ConfigId == x.Config.GetVersionId()
}

// CheckValid will validate that there is at least one state,
// and that the starting state is one of the possible states; further for any of the states it
// will check that they appear in at least one transition.
//
// Finally, it will check that the name is valid,
// and that the generated `ConfigId` is a valid URI segment.
func (x *Configuration) CheckValid() error {
	if x.Name == "" {
		return MissingNameConfigurationError
	}
	if len(x.States) == 0 {
		return MissingStatesConfigurationError
	}
	if x.StartingState == "" || !x.HasState(x.StartingState) {
		return MismatchStartingstateConfigurationError
	}
	// TODO: we should actually build the full graph and check it's fully connected.
	for _, s := range x.States {
		found := false
		for _, t := range x.Transitions {
			if t.HasState(s) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf(UnreachableStateConfigurationError, s)
		}
	}
	return nil
}
