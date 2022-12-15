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
)

func (csm *RedisStore) UpdateState(cfgName string, id string, oldState string, newState string) error {
	var key string
	var err error
	if newState == "" {
		return IllegalStoreError("when updating state, new state cannot be empty")
	}
	if oldState != "" {
		key = NewKeyForMachinesByState(cfgName, oldState)
		err = csm.client.SRem(context.Background(), key, id).Err()
		if err != nil {
			csm.logger.Error("Cannot update FSM [%s#%s] from old state `%s`", cfgName, id, oldState)
			return err
		}
	}
	key = NewKeyForMachinesByState(cfgName, newState)
	err = csm.client.SAdd(context.Background(), key, id).Err()
	if err != nil {
		csm.logger.Error("Cannot update FSM [%s#%s] to new state `%s`", cfgName, id, newState)
	}
	return err
}

func (csm *RedisStore) GetAllInState(cfg string, state string) []string {
	// TODO: enable splitting results with a (cursor, count)
	csm.logger.Debug("Looking up all FSMs [%s] in DB with state `%s`", cfg, state)
	key := NewKeyForMachinesByState(cfg, state)
	fsms, err := csm.client.SMembers(context.Background(), key).Result()
	if err != nil {
		csm.logger.Error("Could not retrieve FSMs for state `%s`: %s", state, err)
		return nil
	}
	csm.logger.Debug("Returning %d items", len(fsms))
	return fsms
}

func (csm *RedisStore) GetAllConfigs() []string {
	// TODO: enable splitting results with a (cursor, count)
	csm.logger.Debug("Looking up all configs in DB")
	configs, err := csm.client.SMembers(context.Background(), ConfigsPrefix).Result()
	if err != nil {
		csm.logger.Error("Could not retrieve configurations: %s", err)
		return nil
	}
	csm.logger.Debug("Returning %d items", len(configs))
	return configs
}

func (csm *RedisStore) GetAllVersions(name string) []string {
	csm.logger.Debug("Looking up all versions for Configurations `%s` in DB", name)
	configs, err := csm.client.SMembers(context.Background(), NewKeyForConfig(name)).Result()
	if err != nil {
		csm.logger.Error("Could not retrieve configurations: %s", err)
		return nil
	}
	csm.logger.Debug("Returning %d items", len(configs))
	return configs
}
