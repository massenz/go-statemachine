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
	"context"
	"fmt"
	. "github.com/JiaYongfei/respect/gomega"
	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/storage"
	slf4go "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RedisStore", func() {

	Context("for simple operations", func() {
		var store storage.StoreManager
		var rdb *redis.Client
		var cfg *protos.Configuration

		BeforeEach(func() {
			cfg = &protos.Configuration{
				Name:          "my_conf",
				Version:       "v3",
				StartingState: "start",
			}
			Expect(container).ToNot(BeNil())
			store = storage.NewRedisStoreWithDefaults(container.Address)
			Expect(store).ToNot(BeNil())
			// Mute unnecessary logging during tests; re-enable (
			// and set to DEBUG) when diagnosing failures.
			store.SetLogLevel(slf4go.NONE)

			// This is used to go "behind the back" of our StoreManager and mess with it for testing
			// purposes. Do NOT do this in your code.
			rdb = redis.NewClient(&redis.Options{
				Addr: container.Address,
				DB:   storage.DefaultRedisDb,
			})
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)
		It("is healthy", func() {
			Expect(store.Health()).To(Succeed())
		})
		It("can get a configuration back", func() {
			id := api.GetVersionId(cfg)
			val, _ := proto.Marshal(cfg)
			res, err := rdb.Set(context.Background(), storage.NewKeyForConfig(id), val,
				storage.NeverExpire).Result()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("OK"))

			data, ok := store.GetConfig(id)
			Expect(ok).To(BeTrue())
			Expect(data).ToNot(BeNil())
			Expect(api.GetVersionId(data)).To(Equal(api.GetVersionId(cfg)))
		})
		It("will return orderly if the id does not exist", func() {
			id := "fake"
			data, ok := store.GetConfig(id)
			Expect(ok).To(BeFalse())
			Expect(data).To(BeNil())
		})
		It("can save configurations", func() {
			var found protos.Configuration
			Expect(store.PutConfig(cfg)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(),
				storage.NewKeyForConfig(api.GetVersionId(cfg))).Bytes()
			Expect(err).ToNot(HaveOccurred())

			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Expect(&found).To(Respect(cfg))
		})
		It("will not save a duplicate configurations", func() {
			Expect(store.PutConfig(cfg)).ToNot(HaveOccurred())
			Expect(store.PutConfig(cfg)).To(HaveOccurred())
		})
		It("should not fail for a non-existent FSM", func() {
			data, ok := store.GetStateMachine("fake", "bad-config")
			Expect(ok).To(BeFalse())
			Expect(data).To(BeNil())
		})
		It("can get an FSM back", func() {
			id := uuid.New().String()
			fsm := &protos.FiniteStateMachine{
				ConfigId: "cfg_id",
				State:    "a-state",
				History:  nil,
			}
			// Storing the FSM behind the store's back
			val, _ := proto.Marshal(fsm)
			key := storage.NewKeyForMachine(id, fsm.ConfigId)
			res, err := rdb.Set(context.Background(), key, val, storage.NeverExpire).Result()

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("OK"))

			data, ok := store.GetStateMachine(id, "cfg_id")
			Expect(ok).To(BeTrue())
			Expect(data).ToNot(BeNil())
			Expect(data).To(Respect(fsm))
		})
		It("can save an FSM", func() {
			id := "99" // uuid.New().String()
			var found protos.FiniteStateMachine
			fsm := &protos.FiniteStateMachine{
				ConfigId: "orders:v4",
				State:    "in_transit",
				History: []*protos.Event{
					{Transition: &protos.Transition{Event: "confirmed"}, Originator: "bot"},
					{Transition: &protos.Transition{Event: "shipped"}, Originator: "bot"},
				},
			}
			Expect(store.PutStateMachine(id, fsm)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(), storage.NewKeyForMachine(id, "orders")).Bytes()
			Expect(err).ToNot(HaveOccurred())

			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			// NOTE: this fails, even though the Protobufs are actually identical:
			//      Expect(found).To(Respect(*fsm))
			// it strangely fails on the History field, which is a slice, and actually matches.
			Expect(found.ConfigId).To(Equal(fsm.ConfigId))
			Expect(found.State).To(Equal(fsm.State))
			Expect(found.ConfigId).To(Equal(fsm.ConfigId))
			Expect(found.History).To(HaveLen(len(fsm.History)))
			Expect(found.History[0]).To(Respect(fsm.History[0]))
			Expect(found.History[1]).To(Respect(fsm.History[1]))
		})
		It("can get events back", func() {
			id := uuid.New().String()
			ev := api.NewEvent("confirmed")
			key := storage.NewKeyForEvent(id, "orders")
			val, _ := proto.Marshal(ev)
			_, err := rdb.Set(context.Background(), key, val, storage.NeverExpire).Result()
			Expect(err).ToNot(HaveOccurred())

			found, ok := store.GetEvent(id, "orders")
			Expect(ok).To(BeTrue())
			Expect(found).To(Respect(ev))
		})
		It("can save events", func() {
			ev := api.NewEvent("confirmed")
			id := ev.EventId
			Expect(store.PutEvent(ev, "orders", storage.NeverExpire)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(), storage.NewKeyForEvent(id, "orders")).Bytes()
			Expect(err).ToNot(HaveOccurred())

			var found protos.Event
			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Expect(&found).To(Respect(ev))
		})
		It("will return an error for a non-existent event", func() {
			_, ok := store.GetEvent("fake", "orders")
			Expect(ok).To(BeFalse())
		})
		It("can save an event Outcome", func() {
			id := uuid.New().String()
			cfg := "orders"
			response := &protos.EventOutcome{
				Code:    protos.EventOutcome_Ok,
				Dest:    "1234-feed-beef",
				Details: "this was just a test",
			}
			Expect(store.AddEventOutcome(id, cfg, response, storage.NeverExpire)).ToNot(HaveOccurred())

			key := storage.NewKeyForOutcome(id, cfg)
			val, err := rdb.Get(context.Background(), key).Bytes()
			Expect(err).ToNot(HaveOccurred())
			var found protos.EventOutcome
			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Expect(&found).To(Respect(response))
		})
		It("can get an event Outcome", func() {
			id := uuid.New().String()
			cfg := "orders"
			response := &protos.EventOutcome{
				Code:    protos.EventOutcome_Ok,
				Dest:    "1234-feed-beef",
				Details: "this was just a test",
			}
			key := storage.NewKeyForOutcome(id, cfg)
			val, _ := proto.Marshal(response)
			_, err := rdb.Set(context.Background(), key, val, storage.NeverExpire).Result()
			Expect(err).ToNot(HaveOccurred())
			found, ok := store.GetOutcomeForEvent(id, cfg)
			Expect(ok).To(BeTrue())
			Expect(found).To(Respect(response))
		})
		It("should gracefully handle a nil Configuration", func() {
			Expect(store.PutConfig(nil)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Statemachine", func() {
			Expect(store.PutStateMachine("fake", nil)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Event", func() {
			Expect(store.PutEvent(nil, "orders", storage.NeverExpire)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Outcome", func() {
			Expect(store.AddEventOutcome("fake", "test", nil,
				storage.NeverExpire)).To(HaveOccurred())
		})
	})

	When("querying for configurations", func() {
		var store storage.StoreManager
		var rdb *redis.Client

		BeforeEach(func() {
			Expect(container).ToNot(BeNil())
			store = storage.NewRedisStoreWithDefaults(container.Address)
			Expect(store).ToNot(BeNil())
			store.SetLogLevel(slf4go.NONE)

			// This is used to go "behind the back" of our StoreManager and mess with it for testing
			// purposes. Do NOT do this in your code.
			rdb = redis.NewClient(&redis.Options{
				Addr: container.Address,
				DB:   storage.DefaultRedisDb,
			})
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)

		It("can get all configuration names", func() {
			for _, name := range []string{"orders", "devices", "users"} {
				Expect(store.PutConfig(&protos.Configuration{Name: name, Version: "v3", StartingState: "start"})).
					ToNot(HaveOccurred())
			}
			configs := store.GetAllConfigs()
			Expect(len(configs)).To(Equal(3))
			Expect(configs).To(ContainElements("orders", "devices", "users"))
		})
		It("can get all versions of a configuration", func() {
			for _, version := range []string{"v1alpha1", "v1beta", "v1"} {
				Expect(store.PutConfig(&protos.Configuration{Name: "orders", Version: version, StartingState: "start"})).
					ToNot(HaveOccurred())
			}
			configs := store.GetAllVersions("orders")
			Expect(len(configs)).To(Equal(3))
			Expect(configs).To(ContainElements("orders:v1alpha1", "orders:v1beta", "orders:v1"))
		})
		It("returns an empty slice for a non-existent config", func() {
			configs := store.GetAllVersions("fake")
			Expect(len(configs)).To(Equal(0))
		})
	})
	When("querying for FSMs", func() {
		var store storage.StoreManager
		var rdb *redis.Client

		BeforeEach(func() {
			Expect(container).ToNot(BeNil())
			store = storage.NewRedisStoreWithDefaults(container.Address)
			Expect(store).ToNot(BeNil())
			store.SetLogLevel(slf4go.NONE)

			// This is used to go "behind the back" of our StoreManager and mess with it for testing
			// purposes. Do NOT do this in your code.
			rdb = redis.NewClient(&redis.Options{
				Addr: container.Address,
				DB:   storage.DefaultRedisDb,
			})
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)
		It("finds them by state", func() {
			for id := 1; id < 5; id++ {
				fsm := &protos.FiniteStateMachine{
					ConfigId: "orders:v4",
					State:    "in_transit",
					History: []*protos.Event{
						{Transition: &protos.Transition{Event: "confirmed"}, Originator: "bot"},
						{Transition: &protos.Transition{Event: "shipped"}, Originator: "bot"},
					},
				}
				fsmId := fmt.Sprintf("fsm-%d", id)
				Expect(store.PutStateMachine(fsmId, fsm)).ToNot(HaveOccurred())
				Expect(store.UpdateState("orders", fsmId, "", fsm.State))
			}
			res := store.GetAllInState("orders", "in_transit")
			Expect(len(res)).To(Equal(4))
			for id := 1; id < 5; id++ {
				Expect(res).To(ContainElement(fmt.Sprintf("fsm-%d", id)))
			}
		})
		When("transitioning state", func() {
			BeforeEach(func() {
				for id := 1; id < 10; id++ {
					fsm := &protos.FiniteStateMachine{
						ConfigId: "orders:v4",
						State:    "in_transit",
						History: []*protos.Event{
							{Transition: &protos.Transition{Event: "confirmed"}, Originator: "bot"},
							{Transition: &protos.Transition{Event: "shipped"}, Originator: "bot"},
						},
					}
					fsmId := fmt.Sprintf("fsm-%d", id)
					Expect(store.PutStateMachine(fsmId, fsm)).ToNot(HaveOccurred())
					Expect(store.UpdateState("orders", fsmId, "", fsm.State))
				}
			})
			It("finds them", func() {
				for id := 3; id < 6; id++ {
					fsmId := fmt.Sprintf("fsm-%d", id)
					Expect(store.UpdateState("orders", fsmId, "in_transit", "shipped"))
				}
				res := store.GetAllInState("orders", "shipped")
				Expect(len(res)).To(Equal(3))
				for id := 3; id < 6; id++ {
					Expect(res).To(ContainElement(fmt.Sprintf("fsm-%d", id)))
				}
				res = store.GetAllInState("orders", "in_transit")
				Expect(len(res)).To(Equal(6))
			})
			It("will remove with an empty newState", func() {
				Expect(store.UpdateState("orders", "fsm-1", "in_transit", "")).To(Succeed())
				res := store.GetAllInState("orders", "in_transit")
				Ω(res).ToNot(ContainElement("fsm-1"))
			})
		})
	})

})
