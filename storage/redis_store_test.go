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
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"

    "context"
    "fmt"
    "github.com/go-redis/redis/v8"
    "github.com/golang/protobuf/proto"
    "github.com/google/uuid"
    log "github.com/massenz/slf4go/logging"
    "github.com/massenz/statemachine-proto/golang/api"
    "os"
    "time"

    . "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/storage"
)

var _ = Describe("RedisStore", func() {

    var redisPort = os.Getenv("REDIS_PORT")
    if redisPort == "" {
        redisPort = storage.DefaultRedisPort
    }

    Context("when configured locally", func() {
        var store storage.StoreManager
        var rdb *redis.Client
        var cfg = &api.Configuration{}

        testTimeout, _ := time.ParseDuration("20ms")
        ctx, _ := context.WithTimeout(context.Background(), testTimeout)

        BeforeEach(func() {
            cfg.Name = "my_conf"
            cfg.Version = "v3"
            cfg.StartingState = "start"

            localAddress := fmt.Sprintf("localhost:%s", redisPort)

            store = storage.NewRedisStore(localAddress, storage.DefaultRedisDb, testTimeout, storage.DefaultMaxRetries)
            Expect(store).ToNot(BeNil())
            store.SetTimeout(testTimeout)
            // Mute unnecessary logging during tests; re-enable (
            // and set to DEBUG) when diagnosing failures.
            store.SetLogLevel(log.NONE)

            // This is used to go "behind the back" of our StoreManager and mess with it for testing
            // purposes. Do NOT do this in your code.
            rdb = redis.NewClient(&redis.Options{
                Addr: localAddress,
                DB:   storage.DefaultRedisDb,
            })
        })

        It("can get a configuration back", func() {
            id := GetVersionId(cfg)
            val, _ := proto.Marshal(cfg)
            res, err := rdb.Set(ctx, storage.NewKeyForConfig(id), val, testTimeout).Result()
            Expect(err).ToNot(HaveOccurred())
            Expect(res).To(Equal("OK"))

            data, ok := store.GetConfig(id)
            Expect(ok).To(BeTrue())
            Expect(data).ToNot(BeNil())
            Expect(GetVersionId(data)).To(Equal(GetVersionId(cfg)))
        })

        It("will return orderly if the id does not exist", func() {
            id := "fake"
            data, ok := store.GetConfig(id)
            Expect(ok).To(BeFalse())
            Expect(data).To(BeNil())
        })

        It("can save configurations", func() {
            var found api.Configuration

            Expect(store.PutConfig(cfg)).ToNot(HaveOccurred())

            val, err := rdb.Get(ctx, GetVersionId(cfg)).Bytes()
            Expect(err).ToNot(HaveOccurred())

            Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
            Expect(found.Name).To(Equal(cfg.Name))
            Expect(found.Version).To(Equal(cfg.Version))
            Expect(found.StartingState).To(Equal(cfg.StartingState))
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
            fsm := &api.FiniteStateMachine{
                ConfigId: "cfg_id",
                State:    "a-state",
                History:  nil,
            }
            // Storing the FSM behind the store's back
            val, _ := proto.Marshal(fsm)
            key := storage.NewKeyForMachine(id, fsm.ConfigId)
            res, err := rdb.Set(ctx, key, val, testTimeout).Result()

            Expect(err).ToNot(HaveOccurred())
            Expect(res).To(Equal("OK"))

            data, ok := store.GetStateMachine(id, "cfg_id")
            Expect(ok).To(BeTrue())
            Expect(data).ToNot(BeNil())
            Expect(data.State).To(Equal(fsm.State))
            Expect(data.ConfigId).To(Equal(fsm.ConfigId))
        })

        It("can save an FSM", func() {
            id := uuid.New().String()
            var found api.FiniteStateMachine
            fsm := &api.FiniteStateMachine{
                ConfigId: "patient.onboard:v3",
                State:    "eligible",
                History: []*api.Event{
                    {Transition: &api.Transition{Event: "started"}, Originator: "bot"},
                    {Transition: &api.Transition{Event: "pending"}, Originator: "bot"},
                },
            }
            Expect(store.PutStateMachine(id, fsm)).ToNot(HaveOccurred())
            val, err := rdb.Get(ctx, storage.NewKeyForMachine(id, "patient.onboard")).Bytes()
            Expect(err).ToNot(HaveOccurred())

            Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
            Expect(found.ConfigId).To(Equal(fsm.ConfigId))
            Expect(found.State).To(Equal(fsm.State))
            for n, evt := range found.History {
                Expect(evt.Transition.Event).To(Equal(fsm.History[n].Transition.Event))
                Expect(evt.Originator).To(Equal("bot"))
            }
        })

        It("should return an error on a nil value store", func() {
            Expect(store.PutConfig(nil)).To(HaveOccurred())
        })
    })
})
