/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package main

import (
	"context"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"gopkg.in/yaml.v3"
)

type HandlersMap = map[string]func(data []byte) (resp interface{}, grpcErr error)

var SendHandlers = HandlersMap{
	KindConfiguration: func(data []byte) (resp interface{}, grpcErr error) {
		var c ConfigEntity
		err := yaml.Unmarshal(data, &c)
		if err != nil {
			return nil, err
		}
		return client.PutConfiguration(context.Background(), c.Spec)
	},
	KindFiniteStateMachine: func(data []byte) (resp interface{}, grpcErr error) {
		var fsm FsmEntity
		err := yaml.Unmarshal(data, &fsm)
		if err != nil {
			return nil, err
		}
		request := &protos.PutFsmRequest{Id: fsm.Id, Fsm: fsm.Spec}
		return client.PutFiniteStateMachine(context.Background(), request)
	},
	KindEvent: func(data []byte) (resp interface{}, grpcErr error) {
		var evt EventRequestEntity
		err := yaml.Unmarshal(data, &evt)
		if err != nil {
			return nil, err
		}
		return SendEvent(evt.Spec)
	},
}
