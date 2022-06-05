package pubsub_test

import (
	"fmt"
	"github.com/massenz/go-statemachine/pubsub"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/massenz/go-statemachine/logging"
)

const (
	timeout            = 5 * time.Second
	eventsQueue        = "test-events"
	notificationsQueue = "test-notifications"
)

func TestPubSub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pub/Sub Suite")
}

// Although these are constants, we cannot take the pointers unless we declare them vars.
var (
	sqsUrl        = "http://localhost:4566"
	region        = "us-west-2"
	testSqsClient = sqs.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Endpoint: &sqsUrl,
			Region:   &region,
		},
	})))
	testLog = logging.NewLog("PUBSUB")
)

var _ = BeforeSuite(func() {
	testLog.Level = logging.NONE
	Expect(os.Setenv("AWS_REGION", region)).ToNot(HaveOccurred())
	for _, topic := range []string{eventsQueue, notificationsQueue} {
		topic = fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())

		_, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		if err != nil {
			// the queue does not exist and ought to be created
			testLog.Info("Creating SQS Queue %s", topic)
			_, err = testSqsClient.CreateQueue(&sqs.CreateQueueInput{
				QueueName: &topic,
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}
})

var _ = AfterSuite(func() {
	for _, topic := range []string{eventsQueue, notificationsQueue} {
		topic = getQueueName(topic)

		out, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		Expect(err).NotTo(HaveOccurred())
		if out != nil {
			testLog.Info("Deleting SQS Queue %s", topic)
			_, err = testSqsClient.DeleteQueue(&sqs.DeleteQueueInput{QueueUrl: out.QueueUrl})
			Expect(err).NotTo(HaveOccurred())
		}
	}
})

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
func postSqsMessage(queue string, msg *pubsub.EventMessage) error {
	q := pubsub.GetQueueUrl(testSqsClient, queue)
	testLog.Debug("Post Message -- Timestamp: %v", msg.EventTimestamp)
	_, err := testSqsClient.SendMessage(&sqs.SendMessageInput{
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"DestinationId": {
				DataType:    aws.String("String"),
				StringValue: aws.String(msg.Destination),
			},
			"EventId": {
				DataType:    aws.String("String"),
				StringValue: aws.String(msg.EventId),
			},
			"Sender": {
				DataType:    aws.String("String"),
				StringValue: aws.String(msg.Sender),
			},
		},
		MessageBody: aws.String(msg.EventName),
		QueueUrl:    &q,
	})
	return err
}
