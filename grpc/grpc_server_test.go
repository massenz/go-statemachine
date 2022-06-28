package grpc_test

import (
    "context"
    "fmt"
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/slf4go/logging"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    g "google.golang.org/grpc"
    "net"
    "time"

    "github.com/massenz/go-statemachine/grpc"
)

var _ = Describe("GrpcServer", func() {

    Context("when configured", func() {

        var testCh chan pubsub.EventMessage
        var listener net.Listener
        var client api.EventsClient
        var done func()

        BeforeEach(func() {
            var err error
            testCh = make(chan pubsub.EventMessage, 5)
            listener, err = net.Listen("tcp", ":0")
            Expect(err).ToNot(HaveOccurred())

            cc, err := g.Dial(listener.Addr().String(), g.WithInsecure())
            Expect(err).ToNot(HaveOccurred())

            client = api.NewEventsClient(cc)
            server, err := grpc.NewGrpcServer(&grpc.Config{
                EventsChannel: testCh,
                Logger:        logging.RootLog,
            })
            Expect(err).ToNot(HaveOccurred())
            Expect(server).ToNot(BeNil())

            go func() {
                server.Serve(listener)
            }()
            done = func() {
                server.Stop()
                cc.Close()
                listener.Close()
            }
        })

        It("should process events", func() {
            response, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    EventId: "1",
                    Transition: &api.Transition{
                        Event: "test-vt",
                    },
                    Originator: "test",
                },
                Dest: "2",
            })
            Expect(err).ToNot(HaveOccurred())
            Expect(response.Ok).To(BeTrue())
            done()
            select {
            case evt := <-testCh:
                Expect(evt.EventId).To(Equal("1"))
                Expect(evt.EventName).To(Equal("test-vt"))
                Expect(evt.Sender).To(Equal("test"))
                Expect(evt.Destination).To(Equal("2"))
            case <-time.After(10 * time.Millisecond):
                Fail("Timed out")

            }
        })

        It("should create an ID for events without", func() {
            response, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    Transition: &api.Transition{
                        Event: "test-vt",
                    },
                    Originator: "test",
                },
                Dest: "123456",
            })
            Expect(err).ToNot(HaveOccurred())
            Expect(response.Ok).To(BeTrue())
            done()
            select {
            case evt := <-testCh:
                Expect(evt.EventId).ToNot(BeEmpty())
                Expect(evt.EventName).To(Equal("test-vt"))
            case <-time.After(10 * time.Millisecond):
                Fail("Timed out")
            }
        })
        It("should fail for missing destination", func() {
            response, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    Transition: &api.Transition{
                        Event: "test-vt",
                    },
                    Originator: "test",
                },
            })
            Expect(err).To(HaveOccurred())
            Expect(response).To(BeNil())
            done()
            select {
            case evt := <-testCh:
                Fail(fmt.Sprintf("Unexpected event: %s", evt))
            case <-time.After(10 * time.Millisecond):
                Succeed()
            }
        })
        It("should fail for missing event", func() {
            response, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    Transition: &api.Transition{
                        Event: "",
                    },
                    Originator: "test",
                },
                Dest: "9876",
            })
            Expect(err).To(HaveOccurred())
            Expect(response).To(BeNil())
            done()
            select {
            case evt := <-testCh:
                Fail(fmt.Sprintf("Unexpected event: %s", evt))
            case <-time.After(10 * time.Millisecond):
                Succeed()
            }
        })
    })

})
