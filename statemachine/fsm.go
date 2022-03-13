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

package statemachine

import (
	"gopkg.in/yaml.v3"
)

func (machine *FSM) State() State {
	return machine.state
}

func (machine *FSM) History() []Event {
	return machine.history
}

func NewFSM(configuration *Configuration) *FSM {
	return &FSM{
		configuration: configuration,
		state:         configuration.StartingState,
		history:       make([]Event, 0),
	}
}

func (machine *FSM) SendEvent(evt Event) error {
	for _, t := range machine.configuration.Transitions {
		if t.From == machine.state && t.Event == evt {
			machine.state = t.To
			machine.history = append(machine.history, evt)
			return nil
		}
	}
	return UnexpectedTransitionError
}

func (machine *FSM) Reset() {
	machine.state = machine.configuration.StartingState
}

// Decode reads the contents as a YAML description of the `Configuration` and
// initializes the FSM accordingly.
//
// The YAML may optionally describe a `SerializedFSM` description and further initialize
// the FSM's current state with the saved state and restore the events' history.
func Decode(content []byte) (*FSM, error) {
	var config SerializedFSM
	err := yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	newFsm := NewFSM(&config.Configuration)
	if newFsm == nil {
		return nil, DecodeError
	}

	if config.CurrentState != "" {
		newFsm.state = config.CurrentState
	}

	if config.SavedHistory != nil {
		for _, evt := range config.SavedHistory {
			newFsm.history = append(newFsm.history, evt)
		}
	}

	return newFsm, nil
}
