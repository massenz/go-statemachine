/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
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

// main simulates a Client sending an SQS event message for an Order entity
// whose status is being tracked by `sm-server`.
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

	// This is the object you want to send across as Event's metadata.
	order := NewOrderDetails(uuid.NewString(), "sqs-cust-1234", 99.99)

	msg := &protos.EventRequest{
		Event: &protos.Event{
			// This is actually unnecessary; if no EventId is present, SM will
			// generate one automatically and if the client does not need to store
			// it somewhere else, it is safe to omit it.
			EventId: uuid.NewString(),

			// This is also unnecessary, as SM will automatically generate a timestamp
			// if one is not already present.
			Timestamp:  timestamppb.Now(),
			Transition: &protos.Transition{Event: *event},
			Originator: "New SQS Client with Details",

			// Here you convert the Event metadata to a string by, e.g., JSON-serializing it.
			Details: order.String(),
		},

		// This is the unique ID for the entity you are sending the event to; MUST
		// match the `id` of an existing `statemachine` (see the REST API).
		Dest: *fsmId,
	}

	_, err = queue.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(proto.MarshalTextString(msg)),
		QueueUrl:    queueUrl.QueueUrl,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Sent event [%s] to queue %s\n", msg.Event.EventId, *q)
}
