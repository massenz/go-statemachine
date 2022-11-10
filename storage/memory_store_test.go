/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package storage_test

import (
	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/storage"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("InMemory Store", func() {
	Context("for local testing", func() {
	})
	Context("can be used to save and retrieve a Configuration", func() {
		var store storage.StoreManager
		var cfg *protos.Configuration

		BeforeEach(func() {
			store = storage.NewInMemoryStore()
			cfg = &protos.Configuration{
				Name:          "my_conf",
				Version:       "v3",
				StartingState: "start",
			}
			Expect(store.PutConfig(cfg)).ToNot(HaveOccurred())
		})
		It("can be created", func() {
			Expect(store).ToNot(BeNil())
		})
		It("will give back the saved Configuration", func() {
			found, ok := store.GetConfig(api.GetVersionId(cfg))
			Expect(ok).To(BeTrue())
			Expect(found).To(Respect(cfg))
		})
		It("will not allow to overwrite an existing config", func() {
			Expect(store.PutConfig(&protos.Configuration{
				Name:          "my_conf",
				Version:       "v3",
				StartingState: "fake",
			})).To(HaveOccurred())
		})
		It("will allow a different version", func() {
			Expect(store.PutConfig(&protos.Configuration{
				Name:          "my_conf",
				Version:       "v4",
				StartingState: "fake",
			})).ToNot(HaveOccurred())
		})
	})
	Context("can be used to save and retrieve a StateMachine", func() {
		var store storage.StoreManager
		var id string
		var machine *protos.FiniteStateMachine

		BeforeEach(func() {
			store = storage.NewInMemoryStore()
			id = "1234"
			machine = &protos.FiniteStateMachine{
				ConfigId: "test:v1",
				State:    "start",
				History:  nil,
			}
			Expect(store.PutStateMachine(id, machine)).ToNot(HaveOccurred())
		})
		It("will give it back unchanged", func() {
			found, ok := store.GetStateMachine(id, "test")
			Expect(ok).To(BeTrue())
			Expect(found).ToNot(BeNil())
			Expect(found.ConfigId).To(Equal(machine.ConfigId))
			Expect(found.History).To(Equal(machine.History))
			Expect(found.State).To(Equal(machine.State))
		})
		It("will return nil for a non-existent id", func() {
			found, ok := store.GetStateMachine("fake", "test")
			Expect(ok).To(BeFalse())
			Expect(found).To(BeNil())
		})
		It("will return an error for a nil FSM", func() {
			machine.ConfigId = "missing"
			Expect(store.PutStateMachine(id, nil)).To(HaveOccurred())
		})
	})
	Context("can be used to save and retrieve Events", func() {
		var store = storage.NewInMemoryStore()
		var id = "1234"
		var event = &protos.Event{
			EventId:    id,
			Timestamp:  timestamppb.Now(),
			Transition: &protos.Transition{Event: "start"},
			Originator: "test",
			Details:    "some details",
		}
		BeforeEach(func() {
			Expect(store.PutEvent(event, "test-cfg", storage.NeverExpire)).ToNot(HaveOccurred())
		})
		It("will give it back unchanged", func() {
			found, ok := store.GetEvent(id, "test-cfg")
			Expect(ok).To(BeTrue())
			Expect(found).ToNot(BeNil())
			Expect(found).To(Respect(event))
		})
		It("will return false for a non-existent id", func() {
			_, ok := store.GetEvent("fake", "test-cfg")
			Expect(ok).To(BeFalse())
		})
	})
})
