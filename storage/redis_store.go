/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
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
	NeverExpire         = 0
	DefaultRedisDb      = 0
	DefaultMaxRetries   = 3
	DefaultTimeout      = 200 * time.Millisecond
	ReturningItemsFmt   = "Returning %d items"
	NoConfigurationsFmt = "Could not retrieve configurations: %s"
)

type RedisStore struct {
	logger     *slf4go.Log
	client     redis.UniversalClient
	Timeout    time.Duration
	MaxRetries int
}

/////// Internal methods

// get abstracts away the common functionality of looking for a key in Redis,
// with a given timeout and a number of retries.
func (csm *RedisStore) get(key string, value proto.Message) StoreErr {
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
			cancel()
			return NotFoundError(key)
		} else if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				// The error here may be recoverable, so we'll keep trying until we run out of attempts
				csm.logger.Error(err.Error())
				if attemptsLeft == 0 {
					csm.logger.Error("max retries reached, giving up")
					cancel()
					return err
				}
				csm.logger.Trace("retrying after timeout, attempts left: %d", attemptsLeft)
				csm.wait()
			} else {
				// This is a different error, we'll just return it
				csm.logger.Error(err.Error())
				cancel()
				return GenericStoreError(err.Error())
			}
		} else {
			cancel()
			return proto.Unmarshal(data, value)
		}
	}
}

func (csm *RedisStore) put(key string, value proto.Message, ttl time.Duration) StoreErr {
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
			return InvalidDataError(err.Error())
		}
		cmd := csm.client.Set(ctx, key, data, ttl)
		_, err = cmd.Result()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				// The error here may be recoverable, so we'll keep trying until we run out of attempts
				if attemptsLeft == 0 {
					return TooManyAttempts("")
				}
				csm.logger.Debug("retrying after timeout, attempts left: %d", attemptsLeft)
				csm.wait()
			} else {
				return GenericStoreError(err.Error())
			}
		} else {
			csm.logger.Debug("stored value for key `%s`", key)
			return nil
		}
	}
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

/////// StoreManager implementation

// Health checks that the server is ready to accept connections
func (csm *RedisStore) Health() StoreErr {
	ctx, cancel := context.WithTimeout(context.Background(), csm.Timeout)
	defer cancel()

	_, err := csm.client.Ping(ctx).Result()
	if err != nil {
		csm.logger.Error("Error pinging redis: %s", err.Error())
		return GenericStoreError(err.Error())
	}
	return nil
}

func (csm *RedisStore) SetTimeout(duration time.Duration) {
	csm.Timeout = duration
}

func (csm *RedisStore) GetTimeout() time.Duration {
	return csm.Timeout
}

// SetLogLevel for RedisStore implements the Loggable interface
func (csm *RedisStore) SetLogLevel(level slf4go.LogLevel) {
	csm.logger.Level = level
}

/////// ConfigStore implementation

func (csm *RedisStore) GetConfig(id string) (*protos.Configuration, StoreErr) {
	key := NewKeyForConfig(id)
	var cfg protos.Configuration
	err := csm.get(key, &cfg)
	if err != nil {
		csm.logger.Error("cannot retrieve configuration: %v", err)
		return nil, err
	}
	return &cfg, nil
}

func (csm *RedisStore) PutConfig(cfg *protos.Configuration) StoreErr {
	if cfg == nil {
		return InvalidDataError("nil config")
	}
	key := NewKeyForConfig(api.GetVersionId(cfg))
	if csm.client.Exists(context.Background(), key).Val() == 1 {
		return AlreadyExistsError(key)
	}
	// TODO: Find out whether the client allows to batch requests, instead of sending multiple server requests
	csm.client.SAdd(context.Background(), ConfigsPrefix, cfg.Name)
	csm.client.SAdd(context.Background(), NewKeyForConfig(cfg.Name), api.GetVersionId(cfg))
	return csm.put(key, cfg, NeverExpire)
}

func (csm *RedisStore) GetAllConfigs() []string {
	// TODO: enable splitting results with a (cursor, count)
	csm.logger.Debug("Looking up all configs in DB")
	configs, err := csm.client.SMembers(context.Background(), ConfigsPrefix).Result()
	if err != nil {
		csm.logger.Error(NoConfigurationsFmt, err)
		return nil
	}
	csm.logger.Debug(ReturningItemsFmt, len(configs))
	return configs
}

func (csm *RedisStore) GetAllVersions(name string) []string {
	csm.logger.Debug("Looking up all versions for Configurations `%s` in DB", name)
	configs, err := csm.client.SMembers(context.Background(), NewKeyForConfig(name)).Result()
	if err != nil {
		csm.logger.Error(NoConfigurationsFmt, err)
		return nil
	}
	csm.logger.Debug(ReturningItemsFmt, len(configs))
	return configs
}

/////// FSMStore implementation

func (csm *RedisStore) GetStateMachine(id string, cfg string) (*protos.FiniteStateMachine, StoreErr) {
	key := NewKeyForMachine(id, cfg)
	var stateMachine protos.FiniteStateMachine
	err := csm.get(key, &stateMachine)
	if err != nil {
		csm.logger.Error("error getting FSM `%s`: %v", key, err)
		return nil, err
	}
	return &stateMachine, nil
}

func (csm *RedisStore) PutStateMachine(id string, stateMachine *protos.FiniteStateMachine) StoreErr {
	if stateMachine == nil {
		return InvalidDataError("nil statemachine")
	}
	configName := strings.Split(stateMachine.ConfigId, api.ConfigurationVersionSeparator)[0]
	key := NewKeyForMachine(id, configName)
	return csm.put(key, stateMachine, NeverExpire)
}

func (csm *RedisStore) GetAllInState(cfg string, state string) []string {
	// TODO: enable splitting results with a (cursor, count)
	csm.logger.Debug("Looking up all FSMs [%s] in DB with state `%s`", cfg, state)
	key := NewKeyForMachinesByState(cfg, state)
	fsms, err := csm.client.SMembers(context.Background(), key).Result()
	if err != nil {
		const format = "Could not retrieve FSMs for state `%s`: %s"
		csm.logger.Error(format, state, err)
		return nil
	}
	csm.logger.Debug(ReturningItemsFmt, len(fsms))
	return fsms
}

func (csm *RedisStore) UpdateState(cfgName string, id string, oldState string, newState string) StoreErr {
	var key string
	var err error
	if oldState != "" {
		key = NewKeyForMachinesByState(cfgName, oldState)
		err = csm.client.SRem(context.Background(), key, id).Err()
		if err != nil {
			return fmt.Errorf(
				"cannot remove FSM [%s#%s] from state set `%s`: %s",
				cfgName, id, oldState, err)
		}
	}
	if newState != "" {
		key = NewKeyForMachinesByState(cfgName, newState)
		err = csm.client.SAdd(context.Background(), key, id).Err()
		if err != nil {
			return fmt.Errorf(
				"cannot add FSM [%s#%s] to state set `%s`: %s",
				cfgName, id, newState, err)
		}
	}
	return nil
}

func (csm *RedisStore) TxProcessEvent(id, cfgName string, evt *protos.Event) StoreErr {
	ctx, _ := context.WithTimeout(context.Background(), DefaultTimeout)
	// See Tx example at https://redis.uptrace.dev/guide/go-redis-pipelines.html#transactions
	txf := func(tx *redis.Tx) error {
		csm.logger.Trace("Tx starts")
		fsm, err := csm.GetStateMachine(id, cfgName)
		if err != nil {
			csm.logger.Debug("error looking up FSM %s: %v", id, err)
			return NotFoundError(NewKeyForMachine(id, cfgName))
		}
		csm.logger.Trace("Tx got SM [%s]", id)
		cfg, err := csm.GetConfig(fsm.ConfigId)
		if err != nil {
			return NotFoundError(err.Error())
		}
		oldState := fsm.GetState()
		csm.logger.Trace("Tx got CFG [%s]", api.GetVersionId(cfg))
		if err = (&api.ConfiguredStateMachine{Config: cfg, FSM: fsm}).SendEvent(evt); err != nil {
			return err
		}
		csm.logger.Trace("Tx changed SM to: %s", fsm.State)
		// If the watched keys are unchanged, the Tx is committed
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			csm.logger.Trace("Tx committing change")
			data, err := proto.Marshal(fsm)
			if err != nil {
				csm.logger.Error("cannot convert proto to bytes: %q", err)
				return InvalidDataError(err.Error())
			}
			cmd := pipe.Set(ctx, NewKeyForMachine(id, cfgName), data, NeverExpire)
			if cmd.Err() != nil {
				csm.logger.Error("could not update fsm [%s](Configuration: %s): %v", id, cfgName, cmd.Err())
				return cmd.Err()
			}
			csm.logger.Trace("Tx committed")
			csm.logger.Trace("updating SET of FSM states")
			err = csm.UpdateState(cfgName, id, oldState, fsm.GetState())
			if err != nil {
				csm.logger.Error("could not update the SET containing FSM per state: %v", err)
				return err
			}
			return nil
		})
		return err
	}
	for i := 0; i < DefaultMaxRetries; i++ {
		key := NewKeyForMachine(id, cfgName)
		csm.logger.Trace("(%d) watching %s", i, key)
		err := csm.client.Watch(ctx, txf, key)
		if err == redis.TxFailedErr {
			// We may be able to retry
			csm.logger.Trace("(%d) Tx failed, retrying", i)
			continue
		}
		// err may be nil here, in which case, success!
		csm.logger.Trace("returning with (%v)", err)
		return err
	}
	return TooManyAttempts("")
}

/////// EventStore implementation

func (csm *RedisStore) GetEvent(id string, cfg string) (*protos.Event, StoreErr) {
	key := NewKeyForEvent(id, cfg)
	var event protos.Event
	err := csm.get(key, &event)
	if err != nil {
		csm.logger.Error("cannot retrieve event `%s`: %v", key, err)
		return nil, err
	}
	return &event, nil
}

func (csm *RedisStore) PutEvent(event *protos.Event, cfg string, ttl time.Duration) StoreErr {
	if event == nil {
		return InvalidDataError("nil event")
	}
	key := NewKeyForEvent(event.EventId, cfg)
	return csm.put(key, event, ttl)
}

func (csm *RedisStore) AddEventOutcome(id string, cfg string, response *protos.EventOutcome, ttl time.Duration) StoreErr {
	if response == nil {
		return InvalidDataError("nil response")
	}
	key := NewKeyForOutcome(id, cfg)
	return csm.put(key, response, ttl)
}

func (csm *RedisStore) GetOutcomeForEvent(id string, cfg string) (*protos.EventOutcome, StoreErr) {
	key := NewKeyForOutcome(id, cfg)
	var outcome protos.EventOutcome
	err := csm.get(key, &outcome)
	if err != nil {
		csm.logger.Error("cannot retrieve outcome for event `%s`: %v", key, err)
		return nil, err
	}
	return &outcome, nil
}

/////// Constructor methods

// NewRedisStoreWithDefaults creates a new StoreManager backed by a Redis server, with
// all default settings, in a single node configuration.
func NewRedisStoreWithDefaults(address string) StoreManager {
	return NewRedisStore(address, false, DefaultRedisDb, DefaultTimeout, DefaultMaxRetries)
}

// NewRedisStore creates a new StoreManager backed by a Redis server, reachable at address, in
// cluster configuration if isCluster is set to true.
// The db value indicates which database to use.
//
// Some store queries (typically the get and put actions) will be retried up to maxRetries times,
// if they time out after timeout expires.
// Use the [Health] function to check whether the store is reachable.
func NewRedisStore(address string, isCluster bool, db int, timeout time.Duration, maxRetries int) StoreManager {
	logger := slf4go.NewLog(fmt.Sprintf("redis://%s/%d", address, db))

	var tlsConfig *tls.Config
	var client redis.UniversalClient

	if os.Getenv("REDIS_TLS") != "" {
		logger.Info("Using TLS for Redis connection")
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	if isCluster {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			TLSConfig: tlsConfig,
			Addrs:     strings.Split(address, ","),
		})
	} else {
		client = redis.NewClient(&redis.Options{
			TLSConfig: tlsConfig,
			Addr:      address,
			DB:        db, // 0 means default DB
		})
	}

	return &RedisStore{
		logger:     slf4go.NewLog(fmt.Sprintf("redis://%s/%d", address, db)),
		client:     client,
		Timeout:    timeout,
		MaxRetries: maxRetries,
	}
}
