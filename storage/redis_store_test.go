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

package storage_test

import (
    . "github.com/JiaYongfei/respect/gomega"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"

    "context"
    "fmt"
    "github.com/go-redis/redis/v8"
    "github.com/golang/protobuf/proto"
    "github.com/google/uuid"
    slf4go "github.com/massenz/slf4go/logging"
    "os"

    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/storage"
    protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("RedisStore", func() {
    var redisPort = os.Getenv("REDIS_PORT")
    if redisPort == "" {
        redisPort = storage.DefaultRedisPort
    }
    localAddress := fmt.Sprintf("localhost:%s", redisPort)

    Context("when configured locally", func() {
        var store storage.StoreManager
        var rdb *redis.Client
        var cfg *protos.Configuration

        BeforeEach(func() {
            cfg = &protos.Configuration{
                Name:          "my_conf",
                Version:       "v3",
                StartingState: "start",
            }
            store = storage.NewRedisStoreWithDefaults(localAddress)
            Expect(store).ToNot(BeNil())
            // Mute unnecessary logging during tests; re-enable (
            // and set to DEBUG) when diagnosing failures.
            store.SetLogLevel(slf4go.NONE)

            // This is used to go "behind the back" of our StoreManager and mess with it for testing
            // purposes. Do NOT do this in your code.
            rdb = redis.NewClient(&redis.Options{
                Addr: localAddress,
                DB:   storage.DefaultRedisDb,
            })
        })
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
            val, err := rdb.Get(context.Background(), api.GetVersionId(cfg)).Bytes()
            Expect(err).ToNot(HaveOccurred())

            Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
            Expect(&found).To(Respect(cfg))
        })
        It("should not save nil values", func() {
            Expect(store.PutConfig(nil)).To(HaveOccurred())
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
            // NOTE: this fails, even though the protos are actually identical:
            //      Expect(found).To(Respect(*fsm))
            // it strangely fails on the History field, which is a slice and it actually matches.
            Expect(found.ConfigId).To(Equal(fsm.ConfigId))
            Expect(found.State).To(Equal(fsm.State))
            Expect(found.ConfigId).To(Equal(fsm.ConfigId))
            Expect(found.History).To(HaveLen(len(fsm.History)))
            Expect(found.History[0]).To(Respect(fsm.History[0]))
            Expect(found.History[1]).To(Respect(fsm.History[1]))
        })
        It("should return an error on a nil value store", func() {
            Expect(store.PutConfig(nil)).To(HaveOccurred())
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
    })
})
