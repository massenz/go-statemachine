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
    "flag"
    "fmt"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/golang/protobuf/proto"
    "github.com/google/uuid"
    "google.golang.org/protobuf/types/known/timestamppb"
    "os"

    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/sqs"

    protos "github.com/massenz/statemachine-proto/golang/api"
)

var CTX = context.TODO()

func NewSqs(endpoint *string) *sqs.SQS {
    region := os.Getenv("AWS_REGION")
    if region == "" {
        region = "us-west-2"
    }

    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
        Config: aws.Config{
            Endpoint: endpoint,
            Region:   &region,
        },
    }))
    return sqs.New(sess)
}

func main() {
    endpoint := flag.String("endpoint", "", "Use http://localhost:4566 to use LocalStack")
    q := flag.String("q", "", "The SQS Queue to send an Event to")
    fsmId := flag.String("dest", "", "The ID for the FSM to send an Event to")
    event := flag.String("evt", "", "The Event for the FSM")
    flag.Parse()

    queue := NewSqs(endpoint)
    queueUrl, err := queue.GetQueueUrl(&sqs.GetQueueUrlInput{
        QueueName: q,
    })
    if err != nil {
        panic(err)
    }

    if *fsmId == "" || *event == "" {
        panic(fmt.Errorf("must specify both of -id and -evt"))
    }
    fmt.Printf("Publishing Event `%s` for FSM `%s` to SQS Topic: [%s]\n", *event, *fsmId, *q)
    order := NewOrderDetails(uuid.NewString(), "sqs-cust-1234", 99.99)
    msg := &protos.EventRequest{
        Event: &protos.Event{
            EventId:    uuid.NewString(),
            Timestamp:  timestamppb.Now(),
            Transition: &protos.Transition{Event: *event},
            Originator: "New SQS Client with Details",
            Details:    order.String(),
        },
        Dest: *fsmId,
    }

    _, err = queue.SendMessage(&sqs.SendMessageInput{
        MessageBody: aws.String(proto.MarshalTextString(msg)),
        QueueUrl:    queueUrl.QueueUrl,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("Sent event to queue", *q)
}
