/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package api_test

import (
	"github.com/golang/protobuf/jsonpb"
	. "github.com/massenz/go-statemachine/pkg/api"
	log "github.com/massenz/slf4go/logging"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("FSM Protocol Buffers", func() {
	BeforeEach(func() {
		Logger = log.NewLog("statemachine-test")
		Logger.Level = log.NONE
	})
	Context("if well defined", func() {
		It("can be initialized", func() {
			var spaceship = protos.Configuration{
				StartingState: "earth",
				States:        []string{"earth", "orbit", "mars"},
				Transitions: []*protos.Transition{
					{From: "earth", To: "orbit", Event: "launch"},
					{From: "orbit", To: "mars", Event: "land"},
				},
			}
			Expect(spaceship.StartingState).To(Equal("earth"))
			Expect(len(spaceship.States)).To(Equal(3))
			Expect(spaceship.Transitions).ToNot(BeEmpty())
			Expect(spaceship.Transitions[0]).ToNot(BeNil())
		})
		It("can be created and used", func() {
			fsm := &protos.FiniteStateMachine{}
			fsm.State = "mars"
			Expect(fsm).ShouldNot(BeNil())
			Expect(fsm.State).Should(Equal("mars"))
			Expect(fsm.History).Should(BeEmpty())
		})
	})

	Context("when defined via JSON", func() {
		var (
			t                                 = protos.Transition{}
			evt                               = protos.Event{}
			gccConfig                         = protos.Configuration{}
			transition, simpleEvent, compiler string
		)

		BeforeEach(func() {
			transition = `{"from": "source", "to": "binary", "event": "build"}`
			simpleEvent = `{"transition": {"event": "build"}}`
			compiler = `{
			"name": "compiler",
			"version": "v1",
			"states": ["source", "tested", "binary"],
			"transitions": [
				{"from": "source", "to": "tested", "event": "test"},
				{"from": "tested", "to": "binary", "event": "build"}
			],
			"starting_state": "source"
		}`
		})

		It("can be parsed without errors", func() {

			Expect(jsonpb.UnmarshalString(transition, &t)).ShouldNot(HaveOccurred())
			Expect(t.From).To(Equal("source"))
			Expect(t.To).To(Equal("binary"))
			Expect(t.Event).To(Equal("build"))
		})
		It("events only need the name of the event to pars", func() {
			Expect(jsonpb.UnmarshalString(simpleEvent, &evt)).ShouldNot(HaveOccurred())
			Expect(evt.Transition.Event).To(Equal("build"))

		})
		It("can define complex configurations", func() {
			Expect(jsonpb.UnmarshalString(compiler, &gccConfig)).ShouldNot(HaveOccurred())
			Expect(len(gccConfig.States)).To(Equal(3))
			Expect(len(gccConfig.Transitions)).To(Equal(2))
			Expect(gccConfig.Version).To(Equal("v1"))
		})
		It("can be used to create FSMs", func() {
			Expect(jsonpb.UnmarshalString(compiler, &gccConfig)).ShouldNot(HaveOccurred())
			fsm, err := NewStateMachine(&gccConfig)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(fsm.FSM.State).To(Equal("source"))
			Expect(fsm.FSM.ConfigId).To(Equal("compiler:v1"))

		})
	})

	Describe("Finite State Machines", func() {
		Context("with a configuration", func() {
			var spaceship protos.Configuration

			BeforeEach(func() {
				spaceship = protos.Configuration{
					StartingState: "earth",
					States:        []string{"earth", "orbit", "mars"},
					Transitions: []*protos.Transition{
						{From: "earth", To: "orbit", Event: "launch"},
						{From: "orbit", To: "mars", Event: "land"},
					},
				}
			})

			It("without name will fail", func() {
				spaceship.Version = "v0.1"
				_, err := NewStateMachine(&spaceship)
				Expect(err).Should(HaveOccurred())
			})
			It("will fail with a missing configuration version", func() {
				spaceship.Name = "mars_orbiter"
				_, err := NewStateMachine(&spaceship)
				Expect(err).To(HaveOccurred())
			})
			It("will carry the configuration embedded", func() {
				spaceship.Name = "mars_orbiter"
				spaceship.Version = "v1.0.1"
				s, err := NewStateMachine(&spaceship)
				Expect(err).ToNot(HaveOccurred())
				Expect(s).ToNot(BeNil())
				Expect(s.Config).ToNot(BeNil())
				Expect(s.Config.String()).To(Equal(spaceship.String()))
				Expect(s.FSM.ConfigId).To(Equal(GetVersionId(&spaceship)))
			})
		})

		Context("with a valid configuration", func() {
			defer GinkgoRecover()
			var spaceship protos.Configuration

			BeforeEach(func() {
				spaceship = protos.Configuration{
					Name:          "mars_rover",
					Version:       "v2.0",
					StartingState: "earth",
					States:        []string{"earth", "orbit", "mars"},
					Transitions: []*protos.Transition{
						{From: "earth", To: "orbit", Event: "launch"},
						{From: "orbit", To: "mars", Event: "land"},
					},
				}
			})
			It("can transition across states ", func() {
				lander, err := NewStateMachine(&spaceship)
				Expect(err).ToNot(HaveOccurred())
				Expect(lander.FSM.State).To(Equal("earth"))
				Expect(lander.SendEvent(NewEvent("launch"))).ShouldNot(HaveOccurred())
				Expect(lander.FSM.State).To(Equal("orbit"))
				Expect(lander.SendEvent(NewEvent("land"))).ShouldNot(HaveOccurred())
				Expect(lander.FSM.State).To(Equal("mars"))
			})
			It("should fail for an unsupported transition", func() {
				lander, _ := NewStateMachine(&spaceship)
				Expect(lander.SendEvent(NewEvent("navigate"))).Should(HaveOccurred())
			})
			It("can be reset", func() {
				lander, _ := NewStateMachine(&spaceship)
				Expect(lander.SendEvent(NewEvent("launch"))).ShouldNot(HaveOccurred())
				Expect(lander.SendEvent(NewEvent("land"))).ShouldNot(HaveOccurred())
				Expect(lander.FSM.State).To(Equal("mars"))

				// Never mind, Elon, let's go home...
				lander.Reset()
				Expect(lander.FSM.State).To(Equal("earth"))
				Expect(lander.FSM.History).To(BeNil())
			})
		})

		Context("can be configured via complex JSON", func() {
			defer GinkgoRecover()
			var (
				orders     protos.Configuration
				configJson []byte
			)
			BeforeEach(func() {
				var err error
				configJson, err = os.ReadFile("../../data/orders.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(jsonpb.UnmarshalString(string(configJson), &orders)).ToNot(HaveOccurred())
			})
			It("JSON can be unmarshalled", func() {
				Expect(orders.Name).To(Equal("test.orders"))
				Expect(orders.Version).To(Equal("v2"))

			})
			It("can be created and events received", func() {
				fsm, err := NewStateMachine(&orders)
				Expect(err).ToNot(HaveOccurred())
				Expect(fsm.FSM).ToNot(BeNil())
				Expect(fsm.FSM.State).To(Equal("start"))

				Expect(fsm.SendEvent(NewEvent("accept"))).ToNot(HaveOccurred())
				Expect(fsm.SendEvent(NewEvent("ship"))).ToNot(HaveOccurred())

				Expect(fsm.FSM.State).To(Equal("shipping"))
				Expect(len(fsm.FSM.History)).To(Equal(2))
			})
		})
	})
})
