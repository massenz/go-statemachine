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

type FSM struct {
	configuration *Configuration
	state         State
	history       []Event
}

var UnexpectedTransition = fmt.Errorf("unexpected event transition")
