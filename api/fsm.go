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
    "github.com/golang/protobuf/proto"
    "github.com/google/uuid"
    log "github.com/massenz/slf4go/logging"
    tspb "google.golang.org/protobuf/types/known/timestamppb"
    "strings"

    protos "github.com/massenz/statemachine-proto/golang/api"
)

const (
    ConfigurationVersionSeparator = ":"
)

var (
    MalformedConfigurationError   = fmt.Errorf("this configuration cannot be parsed")
    MissingNameConfigurationError = fmt.Errorf(
        "configuration must always specify a name (and optionally a version)")
    MissingStatesConfigurationError = fmt.Errorf(
        "configuration must always specify at least one state")
    MissingDestinationError = fmt.Errorf(
        "event must always have a `Destination` statemachine")
    MissingEventNameError = fmt.Errorf(
        "events must always specify the event type")
    MismatchStartingStateConfigurationError = fmt.Errorf(
        "the StartingState must be one of the possible FSM states")
    EmptyStartingStateConfigurationError = fmt.Errorf("the StartingState must be non-empty")
    UnexpectedTransitionError            = fmt.Errorf("unexpected event transition")
    UnexpectedError                      = fmt.Errorf("the request was malformed")
    UnreachableStateConfigurationError   = "state %s is not used in any of the transitions"

    // Logger is made accessible so that its `Level` can be changed
    // or can be sent to a `NullLog` during testing.
    Logger = log.NewLog("fsm")
)

// ConfiguredStateMachine is the internal representation of an FSM, which
// carries within itself the configuration for ease of use.
type ConfiguredStateMachine struct {
    Config *protos.Configuration
    FSM    *protos.FiniteStateMachine
}

func NewStateMachine(configuration *protos.Configuration) (*ConfiguredStateMachine, error) {
    if configuration.Name == "" || configuration.Version == "" {
        Logger.Error("Missing configuration name")
        return nil, MalformedConfigurationError
    }
    return &ConfiguredStateMachine{
        FSM: &protos.FiniteStateMachine{
            ConfigId: strings.Join([]string{configuration.Name, configuration.Version}, ConfigurationVersionSeparator),
            State:    configuration.StartingState,
        },
        Config: configuration,
    }, nil
}

// SendEvent registers the event with the FSM and effects the transition, if valid.
// It also creates a new Event, and stores in the provided cache.
func (x *ConfiguredStateMachine) SendEvent(evt *protos.Event) error {
    // We need to clone the Event, as we will be mutating it,
    // and storing the pointer in the FSM's `History`:
    // we cannot be sure what the caller is going to do with it.
    newEvent := proto.Clone(evt).(*protos.Event)
    for _, t := range x.Config.Transitions {
        if t.From == x.FSM.State && t.Event == newEvent.Transition.Event {
            newEvent.Transition.From = x.FSM.State
            newEvent.Transition.To = t.To
            x.FSM.State = t.To
            x.FSM.History = append(x.FSM.History, newEvent)
            return nil
        }
    }
    return UnexpectedTransitionError
}

// SendEventAsString is a convenience method which also creates the `Event` proto.
func (x *ConfiguredStateMachine) SendEventAsString(evt string) error {
    for _, t := range x.Config.Transitions {
        if t.From == x.FSM.State && t.Event == evt {
            event := NewEvent(evt)
            event.Transition.From = x.FSM.State
            x.FSM.State = t.To
            event.Transition.To = x.FSM.State
            x.FSM.History = append(x.FSM.History, event)
            return nil
        }
    }
    return UnexpectedTransitionError
}

func NewEvent(evt string) *protos.Event {
    return &protos.Event{
        EventId:    uuid.New().String(),
        Timestamp:  tspb.Now(),
        Transition: &protos.Transition{Event: evt},
    }
}

func (x *ConfiguredStateMachine) Reset() {
    x.FSM.State = x.Config.StartingState
    x.FSM.History = nil
}

func GetVersionId(c *protos.Configuration) string {
    return c.Name + ConfigurationVersionSeparator + c.Version
}

// HasState will check whether a given state is either origin or destination for the Transition
func HasState(transition *protos.Transition, state string) bool {
    return state == transition.GetFrom() || state == transition.GetTo()
}

// CfgHasState checks that `state` is one of the Configuration's `States`
func CfgHasState(configuration *protos.Configuration, state string) bool {
    for _, s := range configuration.States {
        if s == state {
            return true
        }
    }
    return false
}

// CheckValid checks that the Configuration is valid and that the current FSM `state` is one of
// the allowed states in the Configuration.
//
// We also check that the reported FSM ConfigId, matches the Configuration's name, version.
func (x *ConfiguredStateMachine) CheckValid() bool {
    return CheckValid(x.Config) == nil && CfgHasState(x.Config, x.FSM.State) &&
        x.FSM.ConfigId == GetVersionId(x.Config)
}

// CheckValid will validate that there is at least one state,
// and that the starting state is one of the possible states; further for any of the states it
// will check that they appear in at least one transition.
//
// Finally, it will check that the name is valid,
// and that the generated `ConfigId` is a valid URI segment.
func CheckValid(c *protos.Configuration) error {
    if c.Name == "" {
        return MissingNameConfigurationError
    }
    if len(c.States) == 0 {
        return MissingStatesConfigurationError
    }
    if c.StartingState == "" {
        return EmptyStartingStateConfigurationError
    }
    if !CfgHasState(c, c.StartingState) {
        return MismatchStartingStateConfigurationError
    }
    // TODO: we should actually build the full graph and check it's fully connected.
    for _, s := range c.States {
        found := false
        for _, t := range c.Transitions {
            if HasState(t, s) {
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

// UpdateEvent adds the ID and timestamp to the event, if not already set.
func UpdateEvent(event *protos.Event) {
    if event.EventId == "" {
        event.EventId = uuid.NewString()
    }
    if event.Timestamp == nil {
        event.Timestamp = tspb.Now()
    }
}
