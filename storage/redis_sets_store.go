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
	"fmt"
)

const (
	ReturningItemsFmt   = "Returning %d items"
	NoConfigurationsFmt = "Could not retrieve configurations: %s"
)

func (csm *RedisStore) UpdateState(cfgName string, id string, oldState string, newState string) error {
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
