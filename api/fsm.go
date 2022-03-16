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
	log "github.com/massenz/go-statemachine/logging"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

var MalformedConfigurationError = fmt.Errorf("this configuration cannot be parsed")
var UnexpectedTransitionError = fmt.Errorf("unexpected event transition")
var UnexpectedEventError = fmt.Errorf("the event was malformed")
var NotImplementedError = fmt.Errorf("not implemented")

var logger = log.NewLog()

// eventsCache is a local cache to store events while this server is running
// TODO: implement a side-load for misses that are fetched from a backing store.
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
		logger.Error("Missing configuration name")
		return nil, MalformedConfigurationError
	}
	if configuration.Version == "" {
		configuration.Version = "v1"
	}
	return &ConfiguredStateMachine{
		FSM: &FiniteStateMachine{
			ConfigId: configuration.Name + ":" + configuration.Version,
			State:    configuration.StartingState,
			History:  make([]string, 0),
		},
		Config: configuration,
	}, nil
}

// SendEvent registers the event with the FSM and effects the transition, if valid.
// It also creates a new Event, and stores in the provided cache.
func (x *ConfiguredStateMachine) SendEvent(evt string) error {
	for _, t := range x.Config.Transitions {

		if t.From == x.FSM.State && t.Event == evt {
			event := &Event{
				EventId:    uuid.New().String(),
				Timestamp:  tspb.Now(),
				Transition: proto.Clone(t).(*Transition),
			}
			x.FSM.State = t.To
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
}
