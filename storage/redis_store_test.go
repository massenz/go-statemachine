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
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/logging"
	"github.com/massenz/go-statemachine/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	proto2 "google.golang.org/protobuf/proto"
	"os"
	"time"
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

		testTimeout, _ := time.ParseDuration("2s")
		ctx, _ := context.WithTimeout(storage.DefaultContext, testTimeout)

		BeforeEach(func() {
			cfg.Name = "my_conf"
			cfg.Version = "v3"
			cfg.StartingState = "start"

			localAddress := fmt.Sprintf("localhost:%s", redisPort)

			store = storage.NewRedisStore(localAddress, storage.DefaultRedisDb)
			Expect(store).ToNot(BeNil())
			store.SetTimeout(testTimeout)
			// Mute unnecessary logging during tests; re-enable (
			//and set to DEBUG) when diagnosing failures.
			store.SetLogLevel(logging.NONE)

			// This is used to go "behind the back" or our StoreManager and mess with it for testing
			// purposes. Do NOT do this in your code.
			rdb = redis.NewClient(&redis.Options{
				Addr: localAddress,
				DB:   storage.DefaultRedisDb,
			})
		})

		It("can get a configuration back", func() {
			id := "1234"
			val, _ := proto.Marshal(cfg)
			res, err := rdb.Set(ctx, id, val, testTimeout).Result()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("OK"))

			data, ok := store.GetConfig(id)
			Expect(ok).To(BeTrue())
			Expect(data).ToNot(BeNil())
			Expect(data.GetVersionId()).To(Equal(cfg.GetVersionId()))
		})

		It("will return orderly if the id does not exist", func() {
			id := "fake"
			data, ok := store.GetConfig(id)
			Expect(ok).To(BeFalse())
			Expect(data).To(BeNil())
		})

		It("can save configurations", func() {
			var found api.Configuration

			Expect(store.PutConfig(cfg.GetVersionId(), cfg)).ToNot(HaveOccurred())

			val, err := rdb.Get(ctx, cfg.GetVersionId()).Bytes()
			Expect(err).ToNot(HaveOccurred())

			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Expect(found.Name).To(Equal(cfg.Name))
			Expect(found.Version).To(Equal(cfg.Version))
			Expect(found.StartingState).To(Equal(cfg.StartingState))
		})

		It("should not save nil values", func() {
			Expect(store.PutConfig("fake", nil)).To(HaveOccurred())
		})

		It("should not fail for a non-existent FSM", func() {
			id := "fake"
			data, ok := store.GetStateMachine(id)
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
			val, _ := proto2.Marshal(fsm)
			res, err := rdb.Set(ctx, id, val, testTimeout).Result()

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal("OK"))

			data, ok := store.GetStateMachine(id)
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
				History:  []string{"started", "pending"},
			}

			Expect(store.PutStateMachine(id, fsm)).ToNot(HaveOccurred())

			val, err := rdb.Get(ctx, id).Bytes()
			Expect(err).ToNot(HaveOccurred())

			Expect(proto.Unmarshal(val, &found)).ToNot(HaveOccurred())
			Expect(found.ConfigId).To(Equal(fsm.ConfigId))
			Expect(found.State).To(Equal(fsm.State))
			Expect(found.History).To(Equal(fsm.History))
		})

		It("should return an error on a nil value store", func() {
			Expect(store.PutConfig("nil-val", nil)).To(HaveOccurred())
		})
	})
})
