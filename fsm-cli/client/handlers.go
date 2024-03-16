/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package client

import (
	"context"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"gopkg.in/yaml.v3"
)

type HandlerFunc = func(*CliClient, []byte) (interface{}, error)
type HandlersMap = map[string]HandlerFunc

var SendHandlers = HandlersMap{
	KindConfiguration: func(clt *CliClient, data []byte) (resp interface{}, grpcErr error) {
		var c ConfigEntity
		err := yaml.Unmarshal(data, &c)
		if err != nil {
			return nil, err
		}
		return clt.PutConfiguration(context.Background(), c.Spec)
	},
	KindFiniteStateMachine: func(clt *CliClient, data []byte) (resp interface{}, grpcErr error) {
		var fsm FsmEntity
		err := yaml.Unmarshal(data, &fsm)
		if err != nil {
			return nil, err
		}
		request := &protos.PutFsmRequest{Id: fsm.Id, Fsm: fsm.Spec}
		return clt.PutFiniteStateMachine(context.Background(), request)
	},
	KindEvent: func(clt *CliClient, data []byte) (resp interface{}, grpcErr error) {
		var evt EventRequestEntity
		err := yaml.Unmarshal(data, &evt)
		if err != nil {
			return nil, err
		}
		return clt.sendEvent(evt.Spec)
	},
}
