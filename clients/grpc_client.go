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

package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "github.com/google/uuid"
    "github.com/massenz/statemachine-proto/golang/api"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/types/known/timestamppb"
    "time"
)

type OrderDetails struct {
    OrderId    string
    CustomerId string
    OrderDate  time.Time
    OrderTotal float64
}

func main() {
    serverAddr := flag.String("addr", ":4567", "The address (host:port) for the GRPC server")
    fsmId := flag.String("dest", "", "The ID for the FSM to send an Event to")
    event := flag.String("evt", "", "The Event for the FSM")
    flag.Parse()

    if *fsmId == "" || *event == "" {
        panic(fmt.Errorf("must specify both of -id and -evt"))
    }
    fmt.Printf("Publishing Event `%s` for FSM `%s` to gRPC Server: [%s]\n",
        *event, *fsmId, *serverAddr)

    clientOptions := []grpc.DialOption{grpc.WithInsecure()}
    cc, _ := grpc.Dial(*serverAddr, clientOptions...)
    client := api.NewStatemachineServiceClient(cc)

    // Fake order
    order := OrderDetails{
        OrderId:    uuid.New().String(),
        CustomerId: uuid.New().String(),
        OrderDate:  time.Now(),
        OrderTotal: 100.0,
    }
    details, _ := json.Marshal(order)

    response, err := client.ConsumeEvent(context.Background(),
        &api.EventRequest{
            Event: &api.Event{
                EventId:   uuid.NewString(),
                Timestamp: timestamppb.Now(),
                Transition: &api.Transition{
                    Event: *event,
                },
                Details:    string(details),
                Originator: "new gRPC Client with details",
            },
            Dest: *fsmId,
        })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Response: %v\n", response)
}
