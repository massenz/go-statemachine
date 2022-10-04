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
    . "github.com/JiaYongfei/respect/gomega"
    "github.com/google/uuid"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"

    "encoding/json"
    log "github.com/massenz/slf4go/logging"
    "net/http"
    "net/http/httptest"
    "strings"

    . "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/server"
    "github.com/massenz/go-statemachine/storage"

    protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("Event Handlers", func() {
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
    server.SetLogLevel(log.NONE)

    Context("when retrieving an Event", func() {
        var id string
        var evt *protos.Event

        BeforeEach(func() {
            store = storage.NewInMemoryStore()
            store.SetLogLevel(log.NONE)
            server.SetStore(store)

            writer = httptest.NewRecorder()
            evt = NewEvent("test")
            id = evt.EventId
            Expect(store.PutEvent(evt, "test-cfg", storage.NeverExpire)).ToNot(HaveOccurred())
        })
        It("can be retrieved with a valid ID", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, "test-cfg", id}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusOK))

            var result server.EventResponse
            Expect(json.NewDecoder(writer.Body).Decode(&result)).ToNot(HaveOccurred())
            Expect(result.ID).To(Equal(id))
            Expect(result.Event).To(Respect(evt))
        })
        It("with an invalid ID will return Not Found", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, "test-cfg", uuid.NewString()}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)

            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
        It("with a missing Config will (eventually) return Not Found", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, "", "12345"}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            // Note: this is done by the router, automatically, removing the redundant slash
            Expect(writer.Code).To(Equal(http.StatusMovedPermanently))
            newLoc := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, "12345"}, "/")
            Expect(writer.Header().Get("Location")).To(Equal(newLoc))

            req = httptest.NewRequest(http.MethodGet, newLoc, nil)
            writer = httptest.NewRecorder()
            router.ServeHTTP(writer, req)
            // Note: this is done by the router, automatically, removing the redundant slash
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
        It("with gibberish data will still fail gracefully", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, "fake", id}, "/")

            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
    })

    Context("when retrieving an Event Outcome", func() {
        var id string
        var outcome *protos.EventOutcome
        var cfgName = "test-cfg"

        BeforeEach(func() {
            store = storage.NewInMemoryStore()
            store.SetLogLevel(log.NONE)
            server.SetStore(store)

            writer = httptest.NewRecorder()
            id = uuid.NewString()
            outcome = &protos.EventOutcome{
                Code:    protos.EventOutcome_Ok,
                Dest:    "fake-sm",
                Details: "something happened",
            }
            Expect(store.AddEventOutcome(id, cfgName, outcome,
                storage.NeverExpire)).ToNot(HaveOccurred())
        })
        It("can be retrieved with a valid ID", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, server.EventsOutcome, cfgName, id}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusOK))

            var result server.OutcomeResponse
            Expect(json.NewDecoder(writer.Body).Decode(&result)).ToNot(HaveOccurred())
            Expect(result.StatusCode).To(Equal(outcome.Code.String()))
            Expect(result.Message).To(Equal(outcome.Details))
            Expect(result.Destination).To(Equal(outcome.Dest))
        })
        It("with an invalid ID will return Not Found", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, server.EventsOutcome, cfgName, uuid.NewString()}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
        It("with a missing Config will (eventually) return Not Found", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, server.EventsOutcome, "", "12345"}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            // Note: this is done by the router, automatically, removing the redundant slash
            Expect(writer.Code).To(Equal(http.StatusMovedPermanently))
            newLoc := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, server.EventsOutcome, "12345"}, "/")
            Expect(writer.Header().Get("Location")).To(Equal(newLoc))

            req = httptest.NewRequest(http.MethodGet, newLoc, nil)
            writer = httptest.NewRecorder()
            router.ServeHTTP(writer, req)
            // Note: this is done by the router, automatically, removing the redundant slash
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
        It("with gibberish data will still fail gracefully", func() {
            endpoint := strings.Join([]string{server.ApiPrefix,
                server.EventsEndpoint, server.EventsOutcome, "fake", id}, "/")
            req = httptest.NewRequest(http.MethodGet, endpoint, nil)
            router.ServeHTTP(writer, req)
            Expect(writer.Code).To(Equal(http.StatusNotFound))
        })
    })
})
