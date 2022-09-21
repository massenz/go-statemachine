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
    "strings"
)

const (
    ConfigsPrefix = "configs"
    EventsPrefix  = "events"
    FsmPrefix     = "fsm"

    KeyPrefixComponentsSeparator = ":"
    KeyPrefixIDSeparator         = "#"
)

// Here we keep all the key definition for the various Redis collections.

// NewKeyForConfig configs#<config:id>
//
// By convention, the config ID is the `name:version` of the configuration; however,
// this is not enforced here, but rather in the implementation of the stores.
func NewKeyForConfig(id string) string {
    return strings.Join([]string{ConfigsPrefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForMachine fsm:<cfg:name>#<machine:id>
func NewKeyForMachine(id string, cfgName string) string {
    prefix := strings.Join([]string{FsmPrefix, cfgName}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForMachinesByState fsm:<cfg:name>:state#<state>
func NewKeyForMachinesByState(cfgName, state string) string {
    prefix := strings.Join([]string{FsmPrefix, cfgName, "state"}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, state}, KeyPrefixIDSeparator)
}

// NewKeyForEvent events:<cfg:name>#<event:id>
func NewKeyForEvent(id string, cfgName string) string {
    prefix := strings.Join([]string{EventsPrefix, cfgName}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForOutcome events:<cfg:name>:outcome#<event:id>
func NewKeyForOutcome(id string, cfgName string) string {
    prefix := strings.Join([]string{EventsPrefix, cfgName, "outcome"}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}
