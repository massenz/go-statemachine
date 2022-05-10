package pubsub_test

import (
    "encoding/json"
    "fmt"
    . "github.com/JiaYongfei/respect/gomega"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "time"

    log "github.com/massenz/go-statemachine/logging"
    "github.com/massenz/go-statemachine/pubsub"
)

var _ = Describe("SQS Publisher", func() {

    Context("when correctly initialized", func() {
        var (
            testPublisher   *pubsub.SqsPublisher
            notificationsCh chan pubsub.EventErrorMessage
        )
        BeforeEach(func() {
            notificationsCh = make(chan pubsub.EventErrorMessage)
            testPublisher = pubsub.NewSqsPublisher(notificationsCh, &sqsUrl)
            Expect(testPublisher).ToNot(BeNil())
            // Set to DEBUG when diagnosing test failures
            testPublisher.SetLogLevel(log.NONE)
        })
        It("can publish error notifications", func() {
            msg := pubsub.EventMessage{
                Sender:         "me",
                Destination:    "some-fsm",
                EventId:        "feed-beef",
                EventName:      "test-me",
                EventTimestamp: time.Now(),
            }
            detail := "more details about the error"
            notification := pubsub.ErrorMessageWithDetail(fmt.Errorf("this is a test"), &msg, detail)
            done := make(chan interface{})
            go func() {
                defer close(done)
                go testPublisher.Publish(getQueueName(notificationsQueue))

            }()
            notificationsCh <- *notification
            res := getSqsMessage(getQueueName(notificationsQueue))
            Expect(res).ToNot(BeNil())
            body := *res.Body
            var sentMsg pubsub.EventMessage
            Expect(json.Unmarshal([]byte(body), &sentMsg)).ToNot(HaveOccurred())
            Expect(sentMsg).To(Respect(msg))

            close(notificationsCh)
            select {
            case <-done:
                Succeed()

            case <-time.After(timeout):
                Fail("timed out waiting for Publisher to exit")
            }
        })

        It("will terminate gracefully when the notifications channel is closed", func() {
            done := make(chan interface{})
            go func() {
                defer close(done)
                testPublisher.Publish(getQueueName(notificationsQueue))
            }()
            close(notificationsCh)
            select {
            case <-done:
                Succeed()
            case <-time.After(timeout):
                Fail("Publisher did not exit within timeout")
            }
        })

        It("will survive an empty Message", func() {
            go testPublisher.Publish(getQueueName(notificationsQueue))
            notificationsCh <- pubsub.EventErrorMessage{}
            close(notificationsCh)
            getSqsMessage(getQueueName(notificationsQueue))
        })

        It("will send several messages within a reasonable timeframe", func() {
            go testPublisher.Publish(getQueueName(notificationsQueue))
            for i := range [10]int{} {
                msg := pubsub.EventMessage{
                    Sender:         "someone",
                    Destination:    fmt.Sprintf("dest-%d", i),
                    EventId:        fmt.Sprintf("evt-%d", i),
                    EventName:      "many-messages-test",
                    EventTimestamp: time.Now(),
                }
                detail := "more details about the error"
                notificationsCh <- *pubsub.ErrorMessageWithDetail(fmt.Errorf("this is a test"), &msg, detail)
            }
            done := make(chan interface{})
            go func() {
                defer close(done)
                for range [10]int{} {
                    res := getSqsMessage(getQueueName(notificationsQueue))
                    Expect(res).ToNot(BeNil())
                    body := *res.Body
                    var sentMsg pubsub.EventMessage
                    Expect(json.Unmarshal([]byte(body), &sentMsg)).ToNot(HaveOccurred())
                    Expect(sentMsg.EventName).To(Equal("many-messages-test"))
                }
            }()
            close(notificationsCh)
            select {
            case <-done:
                Succeed()
            case <-time.After(timeout):
                Fail("timed out waiting for Publisher to exit")
            }
        })
    })
})
