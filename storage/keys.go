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
    KeyPrefixComponentsSeparator = ":"
    KeyPrefixIDSeparator         = "#"
)

// Here we keep all the key definition for the various Redis collections.

// NewKeyForConfig configs#<name:version
func NewKeyForConfig(id string) string {
    prefix := "configs"
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForMachine fsm:<cfg:name>#<machine:id>
func NewKeyForMachine(id string, cfgName string) string {
    prefix := strings.Join([]string{"fsm", cfgName}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForMachinesByState fsm:<cfg:name>:state#<state>
func NewKeyForMachinesByState(cfgName, state string) string {
    prefix := strings.Join([]string{"fsm", cfgName, "state"}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, state}, KeyPrefixIDSeparator)
}

// NewKeyForEvent events:<cfg:name>#<event:id>
func NewKeyForEvent(id string, cfgName string) string {
    prefix := strings.Join([]string{"events", cfgName}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}

// NewKeyForOutcome events:<cfg:name>:outcome#<event:id>
func NewKeyForOutcome(id string, cfgName string) string {
    prefix := strings.Join([]string{"events", cfgName, "outcome"}, KeyPrefixComponentsSeparator)
    return strings.Join([]string{prefix, id}, KeyPrefixIDSeparator)
}
