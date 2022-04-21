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

package storage

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/logging"
	"time"
)

const (
	NeverExpire      = 0
	DefaultRedisPort = "6379"
	DefaultRedisDb   = 0
)

var (
	// Despite what Go thinks, yeah, this IS a constant
	DefaultTimeout, _ = time.ParseDuration("200ms")
	DefaultContext    = context.Background()
)

type RedisStore struct {
	logger  *logging.Log
	client  *redis.Client
	Timeout time.Duration
}

func (csm *RedisStore) SetTimeout(duration time.Duration) {
	csm.Timeout = duration
}

func NewRedisStore(address string, db int) StoreManager {
	return &RedisStore{
		logger: logging.NewLog(fmt.Sprintf("redis:%s", address)),
		client: redis.NewClient(&redis.Options{
			Addr: address,
			// TODO @MM: understand what other int values mean
			DB: db, // 0 means default DB
		}),
		Timeout: DefaultTimeout,
	}
}

func NewRedisStoreWithCreds(address string, db int, username string, password string) StoreManager {
	return &RedisStore{
		logger: logging.NewLog(fmt.Sprintf("redis:%s", address)),
		client: redis.NewClient(&redis.Options{
			Addr:     address,
			Username: username,
			Password: password,
			DB:       db,
		}),
		Timeout: DefaultTimeout,
	}
}

// GetLog for RedisStore implements the Loggable interface
func (csm *RedisStore) GetLog() *logging.Log {
	return csm.logger
}

func (csm *RedisStore) GetConfig(id string) (*api.Configuration, bool) {
	ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
	defer cancel()

	cmd := csm.client.Get(ctx, id)
	data, err := cmd.Bytes()
	if err == redis.Nil {
		csm.logger.Debug("key '%s' not found", id)
	} else if err != nil {
		csm.logger.Error(err.Error())
	} else {
		var cfg api.Configuration

		if err = cfg.UnmarshalBinary(data); err != nil {
			csm.logger.Error("cannot unmarshal data, %q", err)
		} else {
			return &cfg, true
		}
	}
	return nil, false
}

func (csm *RedisStore) PutConfig(id string, cfg *api.Configuration) (err error) {
	ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
	defer cancel()

	if id == "" {
		csm.logger.Error("Cannot store a configuration with an empty ID")
		return IllegalStoreError
	}

	if cfg == nil {
		csm.logger.Error("Attempting to store a nil configuration (%s)", id)
		return IllegalStoreError
	}
	_, err = csm.client.Set(ctx, id, cfg, NeverExpire).Result()
	return
}

func (csm *RedisStore) GetStateMachine(id string) (cfg *api.FiniteStateMachine, ok bool) {
	ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
	defer cancel()

	cmd := csm.client.Get(ctx, id)
	data, err := cmd.Bytes()
	if err == redis.Nil {
		csm.logger.Debug("key '%s' not found", id)
	} else if err != nil {
		csm.logger.Error(err.Error())
	} else {
		var stateMachine api.FiniteStateMachine

		if err = stateMachine.UnmarshalBinary(data); err != nil {
			csm.logger.Error("cannot unmarshal data, %q", err)
		} else {
			return &stateMachine, true
		}
	}
	return nil, false
}

func (csm *RedisStore) PutStateMachine(id string, stateMachine *api.FiniteStateMachine) (err error) {
	ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
	defer cancel()

	if id == "" {
		csm.logger.Error("Cannot store a statemachine with an empty ID")
		return IllegalStoreError
	}

	if stateMachine == nil {
		csm.logger.Error("Attempting to store a nil statemachine (%s)", id)
		return IllegalStoreError
	}
	_, err = csm.client.Set(ctx, id, stateMachine, NeverExpire).Result()
	return
}
