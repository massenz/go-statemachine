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
    "crypto/tls"
    "fmt"
    "github.com/go-redis/redis/v8"
    "github.com/golang/protobuf/proto"
    slf4go "github.com/massenz/slf4go/logging"
    "math/rand"
    "os"
    "strings"
    "time"

    "github.com/massenz/go-statemachine/api"
    protos "github.com/massenz/statemachine-proto/golang/api"
)

const (
    NeverExpire       = 0
    DefaultRedisPort  = "6379"
    DefaultRedisDb    = 0
    DefaultMaxRetries = 3
    DefaultTimeout    = 200 * time.Millisecond
)

type RedisStore struct {
    logger     *slf4go.Log
    client     *redis.Client
    Timeout    time.Duration
    MaxRetries int
}

func (csm *RedisStore) GetAllInState(cfg string, state string) []*protos.FiniteStateMachine {
    // TODO [#33] Ability to query for all machines in a given state
    csm.logger.Error("Not implemented")
    return nil
}

func (csm *RedisStore) GetConfig(id string) (*protos.Configuration, bool) {
    key := NewKeyForConfig(id)
    var cfg protos.Configuration
    err := csm.get(key, &cfg)
    if err != nil {
        csm.logger.Error("Error retrieving configuration `%s`: %s", id, err.Error())
        return nil, false
    }
    return &cfg, true
}

func (csm *RedisStore) GetEvent(id string, cfg string) (*protos.Event, bool) {
    key := NewKeyForEvent(id, cfg)
    var event protos.Event
    err := csm.get(key, &event)
    if err != nil {
        csm.logger.Error("Error retrieving event `%s`: %s", key, err.Error())
        return nil, false
    }
    return &event, true
}

func (csm *RedisStore) GetStateMachine(id string, cfg string) (*protos.FiniteStateMachine, bool) {
    key := NewKeyForMachine(id, cfg)
    var stateMachine protos.FiniteStateMachine
    err := csm.get(key, &stateMachine)
    if err != nil {
        csm.logger.Error("Error retrieving state machine `%s`: %s", key, err.Error())
        return nil, false
    }
    return &stateMachine, true
}

func (csm *RedisStore) PutConfig(cfg *protos.Configuration) error {
    if cfg == nil {
        return IllegalStoreError
    }
    key := NewKeyForConfig(api.GetVersionId(cfg))
    return csm.put(key, cfg, NeverExpire)
}

func (csm *RedisStore) PutEvent(event *protos.Event, cfg string, ttl time.Duration) error {
    if event == nil {
        return IllegalStoreError
    }
    key := NewKeyForEvent(event.EventId, cfg)
    return csm.put(key, event, ttl)
}

func (csm *RedisStore) PutStateMachine(id string, stateMachine *protos.FiniteStateMachine) error {
    if stateMachine == nil {
        return IllegalStoreError
    }
    configName := strings.Split(stateMachine.ConfigId, api.ConfigurationVersionSeparator)[0]
    key := NewKeyForMachine(id, configName)
    return csm.put(key, stateMachine, NeverExpire)
}

func (csm *RedisStore) AddEventOutcome(id string, cfg string, response *protos.EventOutcome, ttl time.Duration) error {
    if response == nil {
        return IllegalStoreError
    }
    key := NewKeyForOutcome(id, cfg)
    return csm.put(key, response, ttl)
}

func (csm *RedisStore) GetOutcomeForEvent(id string, cfg string) (*protos.EventOutcome, bool) {
    key := NewKeyForOutcome(id, cfg)
    var outcome protos.EventOutcome
    err := csm.get(key, &outcome)
    if err != nil {
        csm.logger.Error("Error retrieving outcome for event `%s`: %s", key, err.Error())
        return nil, false
    }
    return &outcome, true
}

func (csm *RedisStore) SetTimeout(duration time.Duration) {
    csm.Timeout = duration
}

func (csm *RedisStore) GetTimeout() time.Duration {
    return csm.Timeout
}

func NewRedisStoreWithDefaults(address string) StoreManager {
    return NewRedisStore(address, DefaultRedisDb, DefaultTimeout, DefaultMaxRetries)
}

func NewRedisStore(address string, db int, timeout time.Duration, maxRetries int) StoreManager {

    logger := slf4go.NewLog(fmt.Sprintf("redis://%s/%d", address, db))
    var tlsConfig *tls.Config
    if os.Getenv("REDIS_TLS") != "" {
        logger.Info("Using TLS for Redis connection")
        tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
    }
    return &RedisStore{
        logger: logger,
        client: redis.NewClient(&redis.Options{
            TLSConfig: tlsConfig,
            Addr:      address,
            DB:        db, // 0 means default DB
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

// `get` abstracts away the common functionality of looking for a key in Redis,
// with a given timeout and a number of retries.
func (csm *RedisStore) get(key string, value proto.Message) error {
    attemptsLeft := csm.MaxRetries
    csm.logger.Trace("Looking up key `%s` (Max retries: %d)", key, attemptsLeft)
    var cancel context.CancelFunc
    defer func() {
        if cancel != nil {
            cancel()
        }
    }()
    for {
        var ctx context.Context
        ctx, cancel = context.WithTimeout(context.Background(), csm.Timeout)
        attemptsLeft--
        cmd := csm.client.Get(ctx, key)
        data, err := cmd.Bytes()
        if err == redis.Nil {
            // The key isn't there, no point in retrying
            csm.logger.Debug("Key `%s` not found", key)
            return err
        } else if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                // The error here may be recoverable, so we'll keep trying until we run out of attempts
                csm.logger.Error(err.Error())
                if attemptsLeft == 0 {
                    csm.logger.Error("max retries reached, giving up")
                    return err
                }
                csm.logger.Trace("retrying after timeout, attempts left: %d", attemptsLeft)
                csm.wait()
            } else {
                // This is a different error, we'll just return it
                csm.logger.Error(err.Error())
                return err
            }
        } else {
            return proto.Unmarshal(data, value)
        }
    }
}

func (csm *RedisStore) put(key string, value proto.Message, ttl time.Duration) error {
    attemptsLeft := csm.MaxRetries
    csm.logger.Trace("Storing key `%s` (Max retries: %d)", key, attemptsLeft)
    var cancel context.CancelFunc
    defer func() {
        if cancel != nil {
            cancel()
        }
    }()
    for {
        var ctx context.Context
        ctx, cancel = context.WithTimeout(context.Background(), csm.Timeout)
        attemptsLeft--
        data, err := proto.Marshal(value)
        if err != nil {
            csm.logger.Error("cannot convert proto to bytes: %q", err)
            return err
        }
        cmd := csm.client.Set(ctx, key, data, ttl)
        _, err = cmd.Result()
        if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                // The error here may be recoverable, so we'll keep trying until we run out of attempts
                csm.logger.Error(err.Error())
                if attemptsLeft == 0 {
                    csm.logger.Error("max retries reached, giving up")
                    return err
                }
                csm.logger.Trace("retrying after timeout, attempts left: %d", attemptsLeft)
                csm.wait()
            } else {
                return err
            }
        }
        return nil
    }
}

func (csm *RedisStore) Health() error {
    ctx, cancel := context.WithTimeout(context.Background(), csm.Timeout)
    defer cancel()

    _, err := csm.client.Ping(ctx).Result()
    if err != nil {
        csm.logger.Error("Error pinging redis: %s", err.Error())
        return fmt.Errorf("Redis health check failed: %w", err)
    }
    return nil
}

// wait is a helper function that sleeps for a random amount of time between 0 and half second.
// Poor man's backoff.
//
// TODO: should use some form of exponential backoff
// TODO: wait time should be configurable
func (csm *RedisStore) wait() {
    waitForMsec := rand.Intn(500)
    time.Sleep(time.Duration(waitForMsec) * time.Millisecond)

}
