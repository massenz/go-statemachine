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
    "github.com/golang/protobuf/proto"
    . "github.com/massenz/go-statemachine/api"
    slf4go "github.com/massenz/slf4go/logging"
    "github.com/massenz/statemachine-proto/golang/api"
    "math/rand"
    "time"
)

const (
    NeverExpire       = 0
    DefaultRedisPort  = "6379"
    DefaultRedisDb    = 0
    DefaultMaxRetries = 3
)

var (
    // Despite what Go thinks, yeah, this IS a constant
    DefaultTimeout, _ = time.ParseDuration("200ms")
    DefaultContext    = context.Background()
)

type RedisStore struct {
    logger     *slf4go.Log
    client     *redis.Client
    Timeout    time.Duration
    MaxRetries int
}

func (csm *RedisStore) SetTimeout(duration time.Duration) {
    csm.Timeout = duration
}

func (csm *RedisStore) GetTimeout() time.Duration {
    return csm.Timeout
}

func NewRedisStore(address string, db int, timeout time.Duration, maxRetries int) StoreManager {
    logger := slf4go.NewLog(fmt.Sprintf("redis://%s/%d", address, db))
    return &RedisStore{
        logger: logger,
        client: redis.NewClient(&redis.Options{
            Addr: address,
            DB:   db, // 0 means default DB
        }),
        Timeout:    timeout,
        MaxRetries: maxRetries,
    }
}

// FIXME: the "constructor" functions are very similar, the creation pattern will need to be
//  refactored to avoid code duplication.

func NewRedisStoreWithCreds(address string, db int, timeout time.Duration, maxRetries int,
    username string, password string) StoreManager {
    return &RedisStore{
        logger: slf4go.NewLog(fmt.Sprintf("redis:%s", address)),
        client: redis.NewClient(&redis.Options{
            Addr:     address,
            Username: username,
            Password: password,
            DB:       db,
        }),
        Timeout:    timeout,
        MaxRetries: maxRetries,
    }
}

// SetLogLevel for RedisStore implements the Loggable interface
func (csm *RedisStore) SetLogLevel(level slf4go.LogLevel) {
    csm.logger.Level = level
}

func (csm *RedisStore) GetConfig(id string) (*api.Configuration, bool) {
    data, err := csm.get(id)
    if err != nil {
        if err != redis.Nil {
            csm.logger.Error("Error retrieving configuration `%s`: %s", id, err.Error())
        }
        return nil, false
    }
    var cfg api.Configuration
    if err = proto.Unmarshal(data, &cfg); err != nil {
        csm.logger.Error("cannot unmarshal data, %q", err)
        return nil, false
    }
    return &cfg, true
}

// `get` abstracts away the common functionality of looking for a key in Redis,
// with a given timeout and a number of retries.
func (csm *RedisStore) get(id string) ([]byte, error) {
    attemptsLeft := csm.MaxRetries
    csm.logger.Debug("Looking up key `%s` (Max retries: %d)", id, attemptsLeft)
    var cancel context.CancelFunc
    defer func() {
        if cancel != nil {
            cancel()
        }
    }()
    for {
        var ctx context.Context
        ctx, cancel = context.WithTimeout(DefaultContext, csm.Timeout)
        attemptsLeft--
        cmd := csm.client.Get(ctx, id)
        data, err := cmd.Bytes()

        if err == redis.Nil {
            // The key isn't there, no point in retrying
            csm.logger.Debug("Key `%s` not found", id)
            return nil, err
        } else if err != nil && ctx.Err() == context.DeadlineExceeded {
            // The error here may be recoverable, so we'll keep trying until we run out of attempts
            csm.logger.Error(err.Error())
            if attemptsLeft == 0 {
                csm.logger.Error("max retries reached, giving up")
                return nil, err
            }
            csm.logger.Warn("retrying after timeout, attempts left: %d", attemptsLeft)
            // Poor man's backoff - TODO: should use some form of exponential backoff
            waitForMsec := rand.Intn(500)
            time.Sleep(time.Duration(waitForMsec) * time.Millisecond)
        } else {
            return data, nil
        }
    }
}

func (csm *RedisStore) PutConfig(cfg *api.Configuration) (err error) {
    ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
    defer cancel()

    if cfg == nil {
        csm.logger.Error("Storing a nil configuration")
        return IllegalStoreError
    }
    value, err := proto.Marshal(cfg)
    if err != nil {
        csm.logger.Error("cannot marshal data, %q", err)
        return err
    }
    _, err = csm.client.Set(ctx, GetVersionId(cfg), value, NeverExpire).Result()
    return
}

func (csm *RedisStore) GetStateMachine(id string) (cfg *api.FiniteStateMachine, ok bool) {
    data, err := csm.get(id)
    if err != nil {
        csm.logger.Error("Error retrieving statemachine `%s`: %s", id, err.Error())
        return nil, false
    }

    var stateMachine api.FiniteStateMachine
    if err = proto.Unmarshal(data, &stateMachine); err != nil {
        csm.logger.Error("cannot unmarshal data, %q", err)
    }
    return &stateMachine, true
}

func (csm *RedisStore) PutStateMachine(id string, stateMachine *api.FiniteStateMachine) (err error) {
    ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
    defer cancel()

    if id == "" {
        csm.logger.Error("Cannot store a State Machine with an empty ID")
        return IllegalStoreError
    }

    if stateMachine == nil {
        csm.logger.Error("Attempting to store a nil statemachine (%s)", id)
        return IllegalStoreError
    }
    value, err := proto.Marshal(stateMachine)
    if err != nil {
        csm.logger.Error("cannot marshal data, %q", err)
        return err
    }
    _, err = csm.client.Set(ctx, id, value, NeverExpire).Result()
    return
}

func (csm *RedisStore) Health() error {
    ctx, cancel := context.WithTimeout(DefaultContext, csm.Timeout)
    defer cancel()

    _, err := csm.client.Ping(ctx).Result()
    if err != nil {
        csm.logger.Error("Error pinging redis: %s", err.Error())
        return fmt.Errorf("Redis health check failed: %w", err)
    }
    return nil
}
