/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package server_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"
	"encoding/json"
	log "github.com/massenz/slf4go/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/server"
	"github.com/massenz/go-statemachine/storage"

	"github.com/massenz/statemachine-proto/golang/api"
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
				Expect(store.PutConfig(config)).ToNot(HaveOccurred())
				req = httptest.NewRequest(http.MethodPost,
					strings.Join([]string{server.ApiPrefix, server.StatemachinesEndpoint}, "/"),
					ReaderFromRequest(request))
			})

			It("should succeed", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				Expect(writer.Header().Get("Location")).To(Equal(
					strings.Join([]string{server.ApiPrefix, server.StatemachinesEndpoint,
						"test-config", "test-machine"}, "/")))
				response := server.StateMachineResponse{}
				Expect(json.Unmarshal(writer.Body.Bytes(), &response)).ToNot(HaveOccurred())

				Expect(response.ID).To(Equal("test-machine"))
				Expect(response.StateMachine.ConfigId).To(Equal("test-config:v1"))
				Expect(response.StateMachine.State).To(Equal("start"))
			})

			It("should fill the cache", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				_, found := store.GetStateMachine("test-machine", "test-config")
				Expect(found).To(BeTrue())
			})

			It("should store the correct data", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				fsm, found := store.GetStateMachine("test-machine", "test-config")
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
				Expect(store.PutConfig(config)).ToNot(HaveOccurred())
				req = httptest.NewRequest(http.MethodPost,
					strings.Join([]string{server.ApiPrefix, server.StatemachinesEndpoint}, "/"),
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
				_, found := store.GetStateMachine(response.ID, "test-config")
				Expect(found).To(BeTrue())
			})

		})
		Context("with a non-existent configuration", func() {
			BeforeEach(func() {
				request := &server.StateMachineRequest{
					ConfigurationVersion: "test-config:v2",
					ID:                   "1234",
				}
				req = httptest.NewRequest(http.MethodPost,
					strings.Join([]string{server.ApiPrefix, server.StatemachinesEndpoint}, "/"),
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
			store.SetLogLevel(log.NONE)
			endpoint := strings.Join([]string{server.ApiPrefix,
				server.StatemachinesEndpoint, "order.card", id}, "/")
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
			endpoint := strings.Join([]string{server.ApiPrefix,
				server.StatemachinesEndpoint, "foo"}, "/")
			req = httptest.NewRequest(http.MethodGet, endpoint, nil)

			router.ServeHTTP(writer, req)
			Expect(writer.Code).To(Equal(http.StatusNotFound))
		})
		It("with a missing ID will return Not Allowed", func() {
			req = httptest.NewRequest(http.MethodGet, strings.Join([]string{server.ApiPrefix,
				server.StatemachinesEndpoint}, "/"), nil)
			router.ServeHTTP(writer, req)
			Expect(writer.Code).To(Equal(http.StatusMethodNotAllowed))
		})
		It("with gibberish data will still fail gracefully", func() {
			cfg := api.Configuration{}
			Expect(store.PutConfig(&cfg)).ToNot(HaveOccurred())
			endpoint := strings.Join([]string{server.ApiPrefix,
				server.StatemachinesEndpoint, "6789"}, "/")
			req = httptest.NewRequest(http.MethodGet, endpoint, nil)
			router.ServeHTTP(writer, req)
			Expect(writer.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("when the statemachine has events", func() {
		var store storage.StoreManager
		var fsmId = "12345"
		var config *api.Configuration

		BeforeEach(func() {
			writer = httptest.NewRecorder()
			store = storage.NewInMemoryStore()
			server.SetStore(store)
			config = &api.Configuration{
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
			car, _ := NewStateMachine(config)
			Expect(store.PutConfig(config)).To(Succeed())
			Expect(store.PutStateMachine(fsmId, car.FSM)).To(Succeed())
		})

		It("it should show them", func() {
			found, _ := store.GetStateMachine(fsmId, "car")
			car := ConfiguredStateMachine{
				Config: config,
				FSM:    found,
			}
			Expect(car.SendEvent(&api.Event{
				EventId:    "1",
				Timestamp:  timestamppb.Now(),
				Transition: &api.Transition{Event: "start"},
				Originator: "test",
				Details:    "this is a test",
			})).To(Succeed())
			Expect(car.SendEvent(&api.Event{
				EventId:    "2",
				Timestamp:  timestamppb.Now(),
				Transition: &api.Transition{Event: "brake"},
				Originator: "test",
				Details:    "a test is this not",
			})).To(Succeed())
			Expect(store.PutStateMachine(fsmId, car.FSM)).To(Succeed())

			endpoint := strings.Join([]string{server.ApiPrefix, server.StatemachinesEndpoint,
				config.Name, fsmId}, "/")
			req = httptest.NewRequest(http.MethodGet, endpoint, nil)
			router.ServeHTTP(writer, req)
			Expect(writer.Code).To(Equal(http.StatusOK))

			var result server.StateMachineResponse
			Expect(json.NewDecoder(writer.Body).Decode(&result)).To(Succeed())

			Expect(result.ID).To(Equal(fsmId))
			Expect(result.StateMachine).ToNot(BeNil())
			fsm := result.StateMachine
			Expect(fsm.State).To(Equal("slowing"))
			Expect(len(fsm.History)).To(Equal(2))
			var history []*api.Event
			history = fsm.History
			event := history[0]
			Expect(event.EventId).To(Equal("1"))
			Expect(event.Originator).To(Equal("test"))
			Expect(event.Transition.Event).To(Equal("start"))
			Expect(event.Transition.From).To(Equal("stopped"))
			Expect(event.Transition.To).To(Equal("running"))
			Expect(event.Details).To(Equal("this is a test"))
			event = history[1]
			Expect(event.EventId).To(Equal("2"))
			Expect(event.Transition.Event).To(Equal("brake"))
			Expect(event.Transition.To).To(Equal("slowing"))
			Expect(event.Details).To(Equal("a test is this not"))
		})
	})
})
