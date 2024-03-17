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
	"github.com/google/uuid"
	"github.com/massenz/go-statemachine/pkg/api"
	storage2 "github.com/massenz/go-statemachine/pkg/storage"
	slf4go "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"strings"
)

const (
	cfgName  = "orders"
	fsmIdFmt = "fsm-%d"
)

// configId is a constant in all but name
var configId = strings.Join([]string{cfgName, "v4"}, api.ConfigurationVersionSeparator)

func setupStoreRedis() (storage2.StoreManager, *redis.Client) {
	store := storage2.NewRedisStoreWithDefaults(container.Address)
	Ω(store).ToNot(BeNil())
	store.SetLogLevel(slf4go.NONE)

	// This is used to go "behind the back" of our StoreManager and mess with it for testing
	// purposes. Do NOT do this in your code.
	rdb := redis.NewClient(&redis.Options{
		Addr: container.Address,
		DB:   storage2.DefaultRedisDb,
	})
	return store, rdb
}

func storeSomeFSMs(store storage2.StoreManager, count int) {
	for id := 1; id < count; id++ {
		fsm := &protos.FiniteStateMachine{
			ConfigId: configId,
			State:    "in_transit",
			History: []*protos.Event{
				{Transition: &protos.Transition{Event: "confirmed"}, Originator: "bot"},
				{Transition: &protos.Transition{Event: "shipped"}, Originator: "bot"},
			},
		}
		fsmId := fmt.Sprintf(fsmIdFmt, id)
		Ω(store.PutStateMachine(fsmId, fsm)).ToNot(HaveOccurred())
		Ω(store.UpdateState("orders", fsmId, "", fsm.State))
	}
}

var _ = Describe("Redis Store", func() {

	Context("for simple operations", func() {
		var store storage2.StoreManager
		var rdb *redis.Client
		var cfg *protos.Configuration

		BeforeEach(func() {
			cfg = &protos.Configuration{
				Name:          "my_conf",
				Version:       "v3",
				StartingState: "start",
			}
			Ω(container).ToNot(BeNil())
			store, rdb = setupStoreRedis()
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)
		It("is healthy", func() {
			Ω(store.Health()).To(Succeed())
		})
		It("can get a configuration back", func() {
			id := api.GetVersionId(cfg)
			val, _ := proto.Marshal(cfg)
			res, err := rdb.Set(context.Background(), storage2.NewKeyForConfig(id), val,
				storage2.NeverExpire).Result()
			Ω(err).ToNot(HaveOccurred())
			Ω(res).To(Equal("OK"))

			data, err := store.GetConfig(id)
			Ω(err).To(BeNil())
			Ω(data).ToNot(BeNil())
			Ω(api.GetVersionId(data)).To(Equal(api.GetVersionId(cfg)))
		})
		It("will return orderly if the id does not exist", func() {
			id := "fake"
			data, err := store.GetConfig(id)
			Ω(err).ToNot(BeNil())
			Ω(data).To(BeNil())
		})
		It("can save configurations", func() {
			var found protos.Configuration
			Ω(store.PutConfig(cfg)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(),
				storage2.NewKeyForConfig(api.GetVersionId(cfg))).Bytes()
			Ω(err).ToNot(HaveOccurred())

			Ω(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Ω(&found).To(Respect(cfg))
		})
		It("will not save a duplicate configurations", func() {
			Ω(store.PutConfig(cfg)).ToNot(HaveOccurred())
			Ω(store.PutConfig(cfg)).To(HaveOccurred())
		})
		It("should not fail for a non-existent FSM", func() {
			_, err := store.GetStateMachine("fake", "bad-config")
			Ω(err).ToNot(BeNil())
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
			key := storage2.NewKeyForMachine(id, fsm.ConfigId)
			res, err := rdb.Set(context.Background(), key, val, storage2.NeverExpire).Result()

			Ω(err).ToNot(HaveOccurred())
			Ω(res).To(Equal("OK"))

			data, err := store.GetStateMachine(id, "cfg_id")
			Ω(err).To(BeNil())
			Ω(data).ToNot(BeNil())
			Ω(data).To(Respect(fsm))
		})
		It("can save an FSM", func() {
			id := "99" // uuid.New().String()
			var found protos.FiniteStateMachine
			fsm := &protos.FiniteStateMachine{
				ConfigId: configId,
				State:    "in_transit",
				History: []*protos.Event{
					{Transition: &protos.Transition{Event: "confirmed"}, Originator: "bot"},
					{Transition: &protos.Transition{Event: "shipped"}, Originator: "bot"},
				},
			}
			Ω(store.PutStateMachine(id, fsm)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(), storage2.NewKeyForMachine(id, cfgName)).Bytes()
			Ω(err).ToNot(HaveOccurred())

			Ω(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			// NOTE: this fails, even though the Protobufs are actually identical:
			//      Ω(found).To(Respect(*fsm))
			// it strangely fails on the History field, which is a slice, and actually matches.
			Ω(found.ConfigId).To(Equal(fsm.ConfigId))
			Ω(found.State).To(Equal(fsm.State))
			Ω(found.ConfigId).To(Equal(fsm.ConfigId))
			Ω(found.History).To(HaveLen(len(fsm.History)))
			Ω(found.History[0]).To(Respect(fsm.History[0]))
			Ω(found.History[1]).To(Respect(fsm.History[1]))
		})
		It("can get events back", func() {
			id := uuid.New().String()
			ev := api.NewEvent("confirmed")
			key := storage2.NewKeyForEvent(id, cfgName)
			val, _ := proto.Marshal(ev)
			_, err := rdb.Set(context.Background(), key, val, storage2.NeverExpire).Result()
			Ω(err).ToNot(HaveOccurred())

			found, err := store.GetEvent(id, cfgName)
			Ω(err).To(BeNil())
			Ω(found).To(Respect(ev))
		})
		It("can save events", func() {
			ev := api.NewEvent("confirmed")
			id := ev.EventId
			Ω(store.PutEvent(ev, cfgName, storage2.NeverExpire)).ToNot(HaveOccurred())
			val, err := rdb.Get(context.Background(), storage2.NewKeyForEvent(id, cfgName)).Bytes()
			Ω(err).ToNot(HaveOccurred())

			var found protos.Event
			Ω(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Ω(&found).To(Respect(ev))
		})
		It("will return an error for a non-existent event", func() {
			_, err := store.GetEvent("fake", cfgName)
			Ω(err).To(HaveOccurred())
		})
		It("can save an event Outcome", func() {
			id := uuid.New().String()
			cfg := cfgName
			response := &protos.EventOutcome{
				Code:    protos.EventOutcome_Ok,
				Config:  "test",
				Id:      "1234-feed-beef",
				Details: "this was just a test",
			}
			Ω(store.AddEventOutcome(id, cfg, response, storage2.NeverExpire)).ToNot(HaveOccurred())

			key := storage2.NewKeyForOutcome(id, cfg)
			val, err := rdb.Get(context.Background(), key).Bytes()
			Ω(err).ToNot(HaveOccurred())
			var found protos.EventOutcome
			Ω(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Ω(&found).To(Respect(response))
		})
		It("can get an event Outcome", func() {
			id := uuid.New().String()
			cfg := cfgName
			response := &protos.EventOutcome{
				Code:    protos.EventOutcome_Ok,
				Details: "this was just a test",
				Id:      "1234-feed-beef",
			}
			key := storage2.NewKeyForOutcome(id, cfg)
			val, _ := proto.Marshal(response)
			_, err := rdb.Set(context.Background(), key, val, storage2.NeverExpire).Result()
			Ω(err).ToNot(HaveOccurred())
			found, err := store.GetOutcomeForEvent(id, cfg)
			Ω(err).ToNot(HaveOccurred())
			Ω(found).To(Respect(response))
		})
		It("should gracefully handle a nil Configuration", func() {
			Ω(store.PutConfig(nil)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Statemachine", func() {
			Ω(store.PutStateMachine("fake", nil)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Event", func() {
			Ω(store.PutEvent(nil, cfgName, storage2.NeverExpire)).To(HaveOccurred())
		})
		It("should gracefully handle a nil Outcome", func() {
			Ω(store.AddEventOutcome("fake", "test", nil,
				storage2.NeverExpire)).To(HaveOccurred())
		})
	})

	When("querying for configurations", func() {
		var store storage2.StoreManager
		var rdb *redis.Client

		BeforeEach(func() {
			store, rdb = setupStoreRedis()
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)

		It("can get all configuration names", func() {
			for _, name := range []string{cfgName, "devices", "users"} {
				Ω(store.PutConfig(&protos.Configuration{Name: name, Version: "v3", StartingState: "start"})).
					ToNot(HaveOccurred())
			}
			configs := store.GetAllConfigs()
			Ω(len(configs)).To(Equal(3))
			Ω(configs).To(ContainElements(cfgName, "devices", "users"))
		})
		It("can get all versions of a configuration", func() {
			for _, version := range []string{"v1alpha1", "v1beta", "v1"} {
				Ω(store.PutConfig(&protos.Configuration{Name: cfgName, Version: version, StartingState: "start"})).
					ToNot(HaveOccurred())
			}
			configs := store.GetAllVersions(cfgName)
			Ω(len(configs)).To(Equal(3))
			Ω(configs).To(ContainElements("orders:v1alpha1", "orders:v1beta", "orders:v1"))
		})
		It("returns an empty slice for a non-existent config", func() {
			configs := store.GetAllVersions("fake")
			Ω(len(configs)).To(Equal(0))
		})
	})
	When("querying for FSMs", func() {
		var store storage2.StoreManager
		var rdb *redis.Client

		BeforeEach(func() {
			store, rdb = setupStoreRedis()
		}, 0.5)
		AfterEach(func() {
			// Cleaning up the DB to prevent "dirty" store to impact test results
			rdb.FlushDB(context.Background())
		}, 0.2)
		It("finds them by state", func() {
			storeSomeFSMs(store, 5)
			res := store.GetAllInState(cfgName, "in_transit")
			Ω(len(res)).To(Equal(4))
			for id := 1; id < 5; id++ {
				Ω(res).To(ContainElement(fmt.Sprintf(fsmIdFmt, id)))
			}
		})
		When("transitioning state", func() {
			BeforeEach(func() {
				storeSomeFSMs(store, 10)
			})
			It("finds them", func() {
				for id := 3; id < 6; id++ {
					fsmId := fmt.Sprintf(fsmIdFmt, id)
					Ω(store.UpdateState(cfgName, fsmId, "in_transit", "shipped"))
				}
				res := store.GetAllInState(cfgName, "shipped")
				Ω(len(res)).To(Equal(3))
				for id := 3; id < 6; id++ {
					Ω(res).To(ContainElement(fmt.Sprintf(fsmIdFmt, id)))
				}
				res = store.GetAllInState(cfgName, "in_transit")
				Ω(len(res)).To(Equal(6))
			})
			It("will remove with an empty newState", func() {
				Ω(store.UpdateState(cfgName, "fsm-1", "in_transit", "")).To(Succeed())
				res := store.GetAllInState(cfgName, "in_transit")
				Ω(res).ToNot(ContainElement("fsm-1"))
			})
		})
	})

})
