package statemachine

import "io/ioutil"
import "gopkg.in/yaml.v3"

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
	return UnexpectedTransition
}

func NewFSMFromFile(yamlFile string) (*FSM, error) {
	contents, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return nil, err
	}
	var config Configuration
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		return nil, err
	}
	return NewFSM(&config), nil
}
