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

package api_test

import (
	"github.com/golang/protobuf/jsonpb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"

	. "github.com/massenz/go-statemachine/api"
)

var _ = Describe("Protocol buffers", func() {

	var spaceship = Configuration{
		StartingState: "earth",
		States:        []string{"earth", "orbit", "mars"},
		Transitions: []*Transition{
			{From: "earth", To: "orbit", Event: "launch"},
			{From: "orbit", To: "mars", Event: "land"},
		},
	}

	Context("if well defined", func() {
		fsm := &FiniteStateMachine{}
		fsm.State = spaceship.StartingState

		It("should be created without errors", func() {
			Expect(fsm).ShouldNot(BeNil())
			Expect(fsm.State).Should(Equal("earth"))
			Expect(fsm.History).Should(BeEmpty())
		})
	})

	Context("can be parsed from JSON", func() {
		defer GinkgoRecover()

		transition := `{"from": "source", "to": "binary", "event": "build"}`
		var t = Transition{}
		Expect(jsonpb.UnmarshalString(transition, &t)).ShouldNot(HaveOccurred())
		Expect(t.From).To(Equal("source"))
		Expect(t.To).To(Equal("binary"))
		Expect(t.Event).To(Equal("build"))

		simpleEvent := `{"transition": {"event": "build"}}`
		var evt = Event{}
		Expect(jsonpb.UnmarshalString(simpleEvent, &evt)).ShouldNot(HaveOccurred())
		// For now, only the event part should be filled in
		Expect(evt.Transition.Event).To(Equal("build"))

		compiler := `{
			"name": "compiler",
			"version": "v1",
			"states": ["source", "tested", "binary"],
			"transitions": [
				{"from": "source", "to": "tested", "event": "test"},
				{"from": "tested", "to": "binary", "event": "build"}
			],
			"starting_state": "source"
		}`
		gccConfig := Configuration{}
		Expect(jsonpb.UnmarshalString(compiler, &gccConfig)).ShouldNot(HaveOccurred())
		Expect(len(gccConfig.States)).To(Equal(3))
		Expect(len(gccConfig.Transitions)).To(Equal(2))
		Expect(gccConfig.Version).To(Equal("v1"))

		fsm, err := NewStateMachine(&gccConfig)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(fsm.FSM.State).To(Equal("source"))
		Expect(fsm.FSM.ConfigId).To(Equal("compiler:v1"))
	})
})

var _ = Describe("Finite State Machines", func() {

	Context("must have a named configuration", func() {
		defer GinkgoRecover()

		var spaceship = Configuration{
			StartingState: "earth",
			States:        []string{"earth", "orbit", "mars"},
			Transitions: []*Transition{
				{From: "earth", To: "orbit", Event: "launch"},
				{From: "orbit", To: "mars", Event: "land"},
			},
		}
		It("or NewStateMachine() will fail", func() {
			_, err := NewStateMachine(&spaceship)
			Expect(err).Should(HaveOccurred())
		})
		It("will get a default version", func() {
			spaceship.Name = "mars_orbiter"
			s, err := NewStateMachine(&spaceship)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(s.FSM.ConfigId).To(Equal("mars_orbiter:v1"))
		})
		It("will carry the configuration embedded", func() {
			s, _ := NewStateMachine(&spaceship)
			Expect(s.Config.String()).To(Equal(spaceship.String()))
		})
	})

	Context("can transition across states", func() {
		defer GinkgoRecover()

		var spaceship = Configuration{
			Name:          "mars_rover",
			Version:       "v2.0",
			StartingState: "earth",
			States:        []string{"earth", "orbit", "mars"},
			Transitions: []*Transition{
				{From: "earth", To: "orbit", Event: "launch"},
				{From: "orbit", To: "mars", Event: "land"},
			},
		}

		lander, err := NewStateMachine(&spaceship)

		Expect(err).ToNot(HaveOccurred())

		Expect(lander.FSM.State).To(Equal("earth"))
		Expect(lander.SendEvent("launch")).ShouldNot(HaveOccurred())

		Expect(lander.FSM.State).To(Equal("orbit"))
		Expect(lander.SendEvent("land")).ShouldNot(HaveOccurred())

		Expect(lander.FSM.State).To(Equal("mars"))

		It("should fail for an unsupported transition", func() {
			Expect(lander.SendEvent("navigate")).Should(HaveOccurred())
		})
	})

	Context("can be configured via complex JSON", func() {
		defer GinkgoRecover()

		configJson, err := ioutil.ReadFile("../data/orders.json")
		Expect(err).ToNot(HaveOccurred())
		var orders Configuration
		Expect(jsonpb.UnmarshalString(string(configJson), &orders)).ToNot(HaveOccurred())
		Expect(orders.Name).To(Equal("test.orders"))
		Expect(orders.Version).To(Equal("v1"))

		fsm, err := NewStateMachine(&orders)
		Expect(err).ToNot(HaveOccurred())
		Expect(fsm.FSM).ToNot(BeNil())
		Expect(fsm.FSM.State).To(Equal("start"))

		fsm.SendEvent("accepted")
		fsm.SendEvent("shipped")
		Expect(fsm.FSM.State).To(Equal("shipping"))
		Expect(len(fsm.FSM.History)).To(Equal(2))
	})
})
