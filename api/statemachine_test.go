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
    log "github.com/massenz/go-statemachine/logging"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "io/ioutil"

    . "github.com/massenz/go-statemachine/api"
)

var _ = Describe("FSM Protocol Buffers", func() {
    BeforeEach(func() { Logger = log.NullLog })
    Context("if well defined", func() {
        It("can be initialized", func() {
            var spaceship = Configuration{
                StartingState: "earth",
                States:        []string{"earth", "orbit", "mars"},
                Transitions: []*Transition{
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
            fsm := &FiniteStateMachine{}
            fsm.State = "mars"
            Expect(fsm).ShouldNot(BeNil())
            Expect(fsm.State).Should(Equal("mars"))
            Expect(fsm.History).Should(BeEmpty())
        })
    })

    Context("when defined via JSON", func() {
        var (
            t                                 = Transition{}
            evt                               = Event{}
            gccConfig                         = Configuration{}
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
        Context("with an unnamed configuration", func() {
            var spaceship Configuration

            BeforeEach(func() {
                spaceship = Configuration{
                    StartingState: "earth",
                    States:        []string{"earth", "orbit", "mars"},
                    Transitions: []*Transition{
                        {From: "earth", To: "orbit", Event: "launch"},
                        {From: "orbit", To: "mars", Event: "land"},
                    },
                }
            })

            It("without name will fail", func() {
                _, err := NewStateMachine(&spaceship)
                Expect(err).Should(HaveOccurred())
            })
            It("will get a default version, if missing", func() {
                spaceship.Name = "mars_orbiter"
                s, err := NewStateMachine(&spaceship)
                Expect(err).ShouldNot(HaveOccurred())
                Expect(s.FSM.ConfigId).To(Equal("mars_orbiter:v1"))
            })
            It("will carry the configuration embedded", func() {
                spaceship.Name = "mars_orbiter"
                spaceship.Version = "v3"
                s, err := NewStateMachine(&spaceship)
                Expect(err).ToNot(HaveOccurred())
                Expect(s).ToNot(BeNil())
                Expect(s.Config).ToNot(BeNil())
                Expect(s.Config.String()).To(Equal(spaceship.String()))
                Expect(s.FSM.ConfigId).To(Equal(spaceship.GetVersionId()))
            })
        })

        Context("with a valid configuration", func() {
            defer GinkgoRecover()
            var spaceship Configuration

            BeforeEach(func() {
                spaceship = Configuration{
                    Name:          "mars_rover",
                    Version:       "v2.0",
                    StartingState: "earth",
                    States:        []string{"earth", "orbit", "mars"},
                    Transitions: []*Transition{
                        {From: "earth", To: "orbit", Event: "launch"},
                        {From: "orbit", To: "mars", Event: "land"},
                    },
                }
            })
            It("can transition across states ", func() {
                lander, err := NewStateMachine(&spaceship)
                Expect(err).ToNot(HaveOccurred())
                Expect(lander.FSM.State).To(Equal("earth"))
                Expect(lander.SendEvent("launch")).ShouldNot(HaveOccurred())
                Expect(lander.FSM.State).To(Equal("orbit"))
                Expect(lander.SendEvent("land")).ShouldNot(HaveOccurred())
                Expect(lander.FSM.State).To(Equal("mars"))
            })
            It("should fail for an unsupported transition", func() {
                lander, _ := NewStateMachine(&spaceship)
                Expect(lander.SendEvent("navigate")).Should(HaveOccurred())
            })
            It("can be reset", func() {
                lander, _ := NewStateMachine(&spaceship)
                Expect(lander.SendEvent("launch")).ShouldNot(HaveOccurred())
                Expect(lander.SendEvent("land")).ShouldNot(HaveOccurred())
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
                orders     Configuration
                configJson []byte
            )
            BeforeEach(func() {
                var err error
                configJson, err = ioutil.ReadFile("../data/orders.json")
                Expect(err).ToNot(HaveOccurred())
                Expect(jsonpb.UnmarshalString(string(configJson), &orders)).ToNot(HaveOccurred())
            })
            It("JSON can be unmarshalled", func() {
                Expect(orders.Name).To(Equal("test.orders"))
                Expect(orders.Version).To(Equal("v1"))

            })
            It("can be created and events received", func() {
                fsm, err := NewStateMachine(&orders)
                Expect(err).ToNot(HaveOccurred())
                Expect(fsm.FSM).ToNot(BeNil())
                Expect(fsm.FSM.State).To(Equal("start"))

                Expect(fsm.SendEvent("accepted")).ToNot(HaveOccurred())
                Expect(fsm.SendEvent("shipped")).ToNot(HaveOccurred())

                Expect(fsm.FSM.State).To(Equal("shipping"))
                Expect(len(fsm.FSM.History)).To(Equal(2))
            })
        })
    })
})
