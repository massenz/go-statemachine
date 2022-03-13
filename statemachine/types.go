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

import "fmt"

type State string
type Event string

type Transition struct {
	From  State `yaml:"from"`
	To    State `yaml:"to"`
	Event Event `yaml:"event"`
}

type Configuration struct {
	States        []State      `yaml:"states"`
	Transitions   []Transition `yaml:"transitions"`
	StartingState State        `yaml:"starting_state"`
}

// FSM is a Finite State Machine model which starts in the configuration's `StartingState` and
// then transitions across states according to received `Event`s; the sequence of events is
// recorded in the `FSM`'s `history`
type FSM struct {
	configuration *Configuration
	state         State
	history       []Event
}

// SerializedFSM represents an FSM that has been stored while running and thus carries both a
// current state (which may be different from `StartingState`) and a saved `History` that needs to
// be restored.
type SerializedFSM struct {
	Configuration
	CurrentState State   `yaml:"current_state"`
	SavedHistory []Event `yaml:"history"`
}

var UnexpectedTransitionError = fmt.Errorf("unexpected event transition")
var DecodeError = fmt.Errorf("cannot create a new FSM from the content")
