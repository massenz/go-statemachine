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

package server_test

import (
    "bytes"
    "encoding/json"
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "time"

    "github.com/massenz/go-statemachine/server"
)

func ReaderFromRequest(request *server.StateMachineRequest) io.Reader {
    jsonBytes, err := json.Marshal(request)
    Expect(err).ToNot(HaveOccurred())
    return bytes.NewBuffer(jsonBytes)
}

var _ = Describe("Handlers", func() {
    var (
        req    *http.Request
        writer *httptest.ResponseRecorder
        store  storage.StoreManager

        // NOTE: we are using the Router here as we need to correctly also parse
        // the URI for path args (just using the router will not do that)
        // The `router` can be safely set for all the test contexts, once and for all.
        router = server.NewRouter()
    )
    // Disabling verbose logging, as it pollutes test output;
    // set it back to DEBUG when tests fail, and you need to
    // diagnose the failure.
    server.SetLogLevel(log.WARN)

    Context("when creating state machines", func() {
        BeforeEach(func() {
            writer = httptest.NewRecorder()
            store = storage.NewInMemoryStore()
            server.SetStore(store)
        })
        Context("with a valid request", func() {
            BeforeEach(func() {
                request := &server.StateMachineRequest{
                    ID:                   "test-machine",
                    ConfigurationVersion: "test-config:v1",
                }
                config := &api.Configuration{
                    Name:          "test-config",
                    Version:       "v1",
                    States:        nil,
                    Transitions:   nil,
                    StartingState: "start",
                }
                Expect(store.PutConfig("test-config:v1", config)).ToNot(HaveOccurred())
                req = httptest.NewRequest(http.MethodPost, server.StatemachinesEndpoint,
                    ReaderFromRequest(request))
            })

            It("should succeed", func() {
                router.ServeHTTP(writer, req)
                Expect(writer.Code).To(Equal(http.StatusCreated))
                Expect(writer.Header().Get("Location")).To(Equal(
                    server.StatemachinesEndpoint + "/test-machine"))
                response := server.StateMachineResponse{}
                Expect(json.Unmarshal(writer.Body.Bytes(), &response)).ToNot(HaveOccurred())

                Expect(response.ID).To(Equal("test-machine"))
                Expect(response.StateMachine.ConfigId).To(Equal("test-config:v1"))
                Expect(response.StateMachine.State).To(Equal("start"))
            })

            It("should fill the cache", func() {
                router.ServeHTTP(writer, req)
                Expect(writer.Code).To(Equal(http.StatusCreated))
                _, found := store.GetStateMachine("test-machine")
                Expect(found).To(BeTrue())
            })

            It("should store the correct data", func() {
                router.ServeHTTP(writer, req)
                Expect(writer.Code).To(Equal(http.StatusCreated))
                fsm, found := store.GetStateMachine("test-machine")
                Expect(found).To(BeTrue())
                Expect(fsm).ToNot(BeNil())
                Expect(fsm.ConfigId).To(Equal("test-config:v1"))
                Expect(fsm.State).To(Equal("start"))
            })
        })
        Context("without specifying an ID", func() {
            BeforeEach(func() {
                request := &server.StateMachineRequest{
                    ConfigurationVersion: "test-config:v1",
                }
                config := &api.Configuration{
                    Name:          "test-config",
                    Version:       "v1",
                    States:        nil,
                    Transitions:   nil,
                    StartingState: "start",
                }
                Expect(store.PutConfig("test-config:v1", config)).ToNot(HaveOccurred())
                req = httptest.NewRequest(http.MethodPost, server.StatemachinesEndpoint,
                    ReaderFromRequest(request))
            })

            It("should succeed with a newly assigned ID", func() {
                router.ServeHTTP(writer, req)
                Expect(writer.Code).To(Equal(http.StatusCreated))
                location := writer.Header().Get("Location")
                Expect(location).ToNot(BeEmpty())
                response := server.StateMachineResponse{}
                Expect(json.Unmarshal(writer.Body.Bytes(), &response)).ToNot(HaveOccurred())

                Expect(response.ID).ToNot(BeEmpty())

                Expect(strings.HasSuffix(location, response.ID)).To(BeTrue())
                _, found := store.GetStateMachine(response.ID)
                Expect(found).To(BeTrue())
            })

        })
        Context("with a non-existent configuration", func() {
            BeforeEach(func() {
                request := &server.StateMachineRequest{
                    ConfigurationVersion: "test-config:v2",
                    ID:                   "1234",
                }
                req = httptest.NewRequest(http.MethodPost, server.StatemachinesEndpoint,
                    ReaderFromRequest(request))
            })

            It("should fail", func() {
                router.ServeHTTP(writer, req)
                Expect(writer.Code).To(Equal(http.StatusNotFound))
                location := writer.Header().Get("Location")
                Expect(location).To(BeEmpty())
                response := server.StateMachineResponse{}
                Expect(json.Unmarshal(writer.Body.Bytes(), &response)).To(HaveOccurred())
                _, found := store.GetConfig("1234")
                Expect(found).To(BeFalse())
            })
        })
    })

    Context("when retrieving a state machine", func() {
        var id string
        var fsm api.FiniteStateMachine

        BeforeEach(func() {
            store = storage.NewInMemoryStore()
            server.SetStore(store)

            writer = httptest.NewRecorder()
            fsm = api.FiniteStateMachine{
                ConfigId: "order.card:v3",
                State:    "checkout",
                History: []*api.Event{
                    {Transition: &api.Transition{Event: "order_placed"}, Originator: ""},
                    {Transition: &api.Transition{Event: "checked_out"}, Originator: ""},
                },
            }
            id = "12345"
            Expect(store.PutStateMachine(id, &fsm)).ToNot(HaveOccurred())
        })

        It("can be retrieved with a valid ID", func() {
            endpoint := strings.Join([]string{server.StatemachinesEndpoint, id}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusOK))

            var result server.StateMachineResponse
            Expect(json.NewDecoder(writer.Body).Decode(&result)).ToNot(HaveOccurred())

            Expect(result.ID).To(Equal(id))
            sm := result.StateMachine
            Expect(sm.ConfigId).To(Equal(fsm.ConfigId))
            Expect(sm.State).To(Equal(fsm.State))
            Expect(len(sm.History)).To(Equal(len(fsm.History)))
            for n, t := range sm.History {
                Expect(t.Transition.Event).To(Equal(fsm.History[n].Transition.Event))
            }
        })
        It("with an invalid ID will return Not Found", func() {
            endpoint := strings.Join([]string{server.StatemachinesEndpoint, "foo"}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
        It("with a missing ID will return Not Allowed", func() {
            req = httptest.NewRequest(http.MethodGet, server.StatemachinesEndpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusMethodNotAllowed))
        })

        It("with gibberish data will still fail gracefully", func() {
            cfg := api.Configuration{}
            store.PutConfig("6789", &cfg)
            endpoint := strings.Join([]string{server.StatemachinesEndpoint, "6789"}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
    })

    Context("when receiving events", func() {
        // We need a channel to send events to
        var store storage.StoreManager
        var events chan pubsub.EventMessage
        var listener *pubsub.EventsListener

        BeforeEach(func() {
            events = make(chan pubsub.EventMessage)
            writer = httptest.NewRecorder()
            store = storage.NewInMemoryStore()
            server.SetStore(store)

            listener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
                EventsChannel:        events,
                NotificationsChannel: nil,
                StatemachinesStore:   store,
                ListenersPoolSize:    0,
            })
            listener.SetLogLevel(log.NONE)

            config := &api.Configuration{
                Name:    "car",
                Version: "v1",
                States:  []string{"stopped", "running", "slowing"},
                Transitions: []*api.Transition{
                    {From: "stopped", To: "running", Event: "start"},
                    {From: "running", To: "slowing", Event: "brake"},
                    {From: "slowing", To: "running", Event: "accelerate"},
                    {From: "slowing", To: "stopped", Event: "stop"},
                },
                StartingState: "stopped",
            }
            car := &api.FiniteStateMachine{
                ConfigId: "car:v1",
                State:    "stopped",
                History:  nil,
            }
            Expect(store.PutConfig(config.GetVersionId(), config)).ToNot(HaveOccurred())
            Expect(store.PutStateMachine("sm-123", car)).ToNot(HaveOccurred())

        })

        It("it should show them", func() {
            done := make(chan interface{})
            go func() {
                defer close(done)
                go listener.ListenForMessages()
            }()

            fsmId := "sm-123"
            event := pubsub.EventMessage{
                Sender:         "test-sender",
                Destination:    fsmId,
                EventId:        "1",
                EventName:      "start",
                EventTimestamp: time.Time{},
            }
            events <- event

            event.EventId = "2"
            event.EventName = "brake"
            events <- event

            event.EventId = "3"
            event.EventName = "stop"
            events <- event
            close(events)

            // Wait a bit for the events to be fully processed.
            time.Sleep(5 * time.Millisecond)

            endpoint := strings.Join([]string{server.StatemachinesEndpoint, fsmId}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusOK))

            var result server.StateMachineResponse
            Expect(json.NewDecoder(writer.Body).Decode(&result)).ToNot(HaveOccurred())

            Expect(result.ID).To(Equal(fsmId))
            Expect(result.StateMachine).ToNot(BeNil())
            fsm := result.StateMachine
            Expect(fsm.State).To(Equal("stopped"))
            Expect(len(fsm.History)).To(Equal(3))
            var history []*api.Event
            history = fsm.History
            startEvent := history[0]
            Expect(startEvent.EventId).To(Equal("1"))
            Expect(startEvent.Originator).To(Equal("test-sender"))
            Expect(startEvent.Transition.Event).To(Equal("start"))
            Expect(startEvent.Transition.From).To(Equal("stopped"))
            Expect(startEvent.Transition.To).To(Equal("running"))
            brakeEvent := history[1]
            Expect(brakeEvent.EventId).To(Equal("2"))
            Expect(brakeEvent.Transition.Event).To(Equal("brake"))
            Expect(brakeEvent.Transition.To).To(Equal("slowing"))
            stopEvent := history[2]
            Expect(stopEvent.EventId).To(Equal("3"))
            Expect(stopEvent.Transition.Event).To(Equal("stop"))
            Expect(stopEvent.Transition.To).To(Equal("stopped"))

            select {
            case <-done:
                Succeed()
            case <-time.After(100 * time.Millisecond):
                Fail("timed out waiting for Listener to exit")
            }
        })
    })
})
