/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	internals "github.com/massenz/go-statemachine/pkg/internal/testing"
	"github.com/massenz/go-statemachine/pkg/pubsub"
	"github.com/massenz/statemachine-proto/golang/api"
)

const (
	eventsQueue        = "test-events"
	notificationsQueue = "test-notifications"
	timeout            = 1 * time.Second       // Default timeout for Eventually is 1s
	pollingInterval    = 10 * time.Millisecond // Default polling interval for Eventually is 10ms
)

func TestPubSub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pub/Sub Suite")
}

// Although these are constants, we cannot take the pointers unless we declare them vars.
var (
	awsLocal       *internals.Container
	redisContainer *internals.Container
	testSqsClient  *sqs.SQS
)

var _ = BeforeSuite(func() {
	Expect(os.Setenv("AWS_REGION", internals.Region)).ToNot(HaveOccurred())

	var err error
	awsLocal, err = internals.NewLocalstackContainer(context.Background())
	Ω(err).ToNot(HaveOccurred())

	// Can't take the address of a constant.
	region := internals.Region
	testSqsClient = sqs.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Endpoint: &awsLocal.Address,
			Region:   &region,
		},
	})))

	for _, topic := range []string{eventsQueue, notificationsQueue} {
		topic = fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())
		if _, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: &topic}); err != nil {
			// the queue does not exist and ought to be created
			_, err = testSqsClient.CreateQueue(&sqs.CreateQueueInput{QueueName: &topic})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	redisContainer, err = internals.NewRedisContainer(context.Background())
	Ω(err).ToNot(HaveOccurred())
}, 2.0)

var _ = AfterSuite(func() {
	if awsLocal != nil {
		Ω(awsLocal.Terminate(context.Background())).ToNot(HaveOccurred())
	}
	if redisContainer != nil {
		Ω(redisContainer.Terminate(context.Background())).ToNot(HaveOccurred())
	}
}, 2.0)

// getQueueName provides a way to obtain a process-independent name for the SQS queue,
// when Ginkgo tests are run in parallel (-p)
func getQueueName(topic string) string {
	return fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())
}

func getSqsMessage(queue string) *sqs.Message {
	q := pubsub.GetQueueUrl(testSqsClient, queue)
	out, err := testSqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl: &q,
	})
	Expect(err).ToNot(HaveOccurred())
	// It could be that the message is not yet available, so we need to retry
	if len(out.Messages) == 0 {
		return nil
	}
	Expect(len(out.Messages)).To(Equal(1))
	_, err = testSqsClient.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &q,
		ReceiptHandle: out.Messages[0].ReceiptHandle,
	})
	Expect(err).ToNot(HaveOccurred())
	return out.Messages[0]
}

// postSqsMessage mirrors the decoding of the SQS Message in the Subscriber and will
// send it over the `queue`, so that we can test the Publisher can correctly receive it.
func postSqsMessage(queue string, msg *api.EventRequest) error {
	marshaler := pubsub.Base64ProtoMarshaler{}
	body, err := marshaler.MarshalToText(msg)
	Expect(err).ToNot(HaveOccurred())
	q := pubsub.GetQueueUrl(testSqsClient, queue)
	_, err = testSqsClient.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(body)),
		QueueUrl:    &q,
	})
	return err
}
