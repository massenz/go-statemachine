package main

import (
    "context"
    "flag"
    "fmt"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/google/uuid"
    "os"

    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/sqs"
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

    if *fsmId == "" || *event == "" {
        panic(fmt.Errorf("must specify both of -id and -evt"))
    }
    fmt.Printf("Publishing Event `%s` for FSM `%s` to SQS Topic: [%s]\n",
        *event, *fsmId, *q)

    queue := NewSqs(endpoint)
    queueUrl, err := queue.GetQueueUrl(&sqs.GetQueueUrlInput{
        QueueName: q,
    })
    if err != nil {
        panic(err)
    }
    _, err = queue.SendMessage(&sqs.SendMessageInput{
        MessageAttributes: map[string]*sqs.MessageAttributeValue{
            "DestinationId": {
                DataType:    aws.String("String"),
                StringValue: aws.String(*fsmId),
            },
            "EventId": {
                DataType:    aws.String("String"),
                StringValue: aws.String(uuid.NewString()),
            },
            "Sender": {
                DataType:    aws.String("String"),
                StringValue: aws.String("SQS Client"),
            },
        },
        MessageBody: aws.String(*event),
        QueueUrl:    queueUrl.QueueUrl,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("Sent event to queue", *q)
}
