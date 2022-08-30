package grpc_test

import (
    . "github.com/JiaYongfei/respect/gomega"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"

    "context"
    "fmt"
    "github.com/massenz/slf4go/logging"

    g "google.golang.org/grpc"
    "net"
    "time"

    . "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/grpc"
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/go-statemachine/storage"
    "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("GrpcServer", func() {

    Context("when configured", func() {
        var testCh chan pubsub.EventMessage
        var listener net.Listener
        var client api.StatemachineServiceClient
        var done func()
        var store = storage.NewInMemoryStore()
        store.SetLogLevel(logging.NONE)

        BeforeEach(func() {
            var err error
            testCh = make(chan pubsub.EventMessage, 5)
            listener, err = net.Listen("tcp", ":0")
            Ω(err).ShouldNot(HaveOccurred())

            cc, err := g.Dial(listener.Addr().String(), g.WithInsecure())
            Ω(err).ShouldNot(HaveOccurred())

            client = api.NewStatemachineServiceClient(cc)
            l := logging.NewLog("grpc-server-test")
            l.Level = logging.NONE
            server, err := grpc.NewGrpcServer(&grpc.Config{
                EventsChannel: testCh,
                Store:         store,
                Logger:        l,
            })
            Ω(err).ToNot(HaveOccurred())
            Ω(server).ToNot(BeNil())

            go func() {
                Ω(server.Serve(listener)).Should(Succeed())
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
            Ω(err).ToNot(HaveOccurred())
            Ω(response).ToNot(BeNil())
            Ω(response.EventId).To(Equal("1"))
            done()
            select {
            case evt := <-testCh:
                Ω(evt.EventId).To(Equal("1"))
                Ω(evt.EventName).To(Equal("test-vt"))
                Ω(evt.Sender).To(Equal("test"))
                Ω(evt.Destination).To(Equal("2"))
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
            Ω(err).ToNot(HaveOccurred())
            Ω(response.EventId).ToNot(BeNil())
            generatedId := response.EventId
            done()
            select {
            case evt := <-testCh:
                Ω(evt.EventId).Should(Equal(generatedId))
                Ω(evt.EventName).To(Equal("test-vt"))
            case <-time.After(10 * time.Millisecond):
                Fail("Timed out")
            }
        })
        It("should fail for missing destination", func() {
            _, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    Transition: &api.Transition{
                        Event: "test-vt",
                    },
                    Originator: "test",
                },
            })
            Ω(err).To(HaveOccurred())
            done()
            select {
            case evt := <-testCh:
                Fail(fmt.Sprintf("Unexpected event: %s", evt))
            case <-time.After(10 * time.Millisecond):
                Succeed()
            }
        })
        It("should fail for missing event", func() {
            _, err := client.ConsumeEvent(context.Background(), &api.EventRequest{
                Event: &api.Event{
                    Transition: &api.Transition{
                        Event: "",
                    },
                    Originator: "test",
                },
                Dest: "9876",
            })
            Ω(err).To(HaveOccurred())
            done()
            select {
            case evt := <-testCh:
                Fail(fmt.Sprintf("UnΩed event: %s", evt))
            case <-time.After(10 * time.Millisecond):
                Succeed()
            }
        })

        // Store management tests
        var cfg *api.Configuration
        var fsm *api.FiniteStateMachine
        BeforeEach(func() {
            cfg = &api.Configuration{
                Name:    "test-conf",
                Version: "v1",
                States:  []string{"start", "stop"},
                Transitions: []*api.Transition{
                    {From: "start", To: "stop", Event: "shutdown"},
                },
                StartingState: "start",
            }
            fsm = &api.FiniteStateMachine{ConfigId: GetVersionId(cfg)}
        })
        It("should store valid configurations", func() {
            _, ok := store.GetConfig(GetVersionId(cfg))
            Ω(ok).To(BeFalse())
            response, err := client.PutConfiguration(context.Background(), cfg)
            Ω(err).ToNot(HaveOccurred())
            Ω(response).ToNot(BeNil())
            Ω(response.Id).To(Equal(GetVersionId(cfg)))
            found, ok := store.GetConfig(response.Id)
            Ω(ok).Should(BeTrue())
            Ω(found).Should(Respect(cfg))
        })
        It("should fail for invalid configuration", func() {
            invalid := &api.Configuration{
                Name:          "invalid",
                Version:       "v1",
                States:        []string{},
                Transitions:   nil,
                StartingState: "",
            }
            _, err := client.PutConfiguration(context.Background(), invalid)
            Ω(err).To(HaveOccurred())
        })
        It("should retrieve a valid configuration", func() {
            Ω(store.PutConfig(cfg)).To(Succeed())
            response, err := client.GetConfiguration(context.Background(),
                &api.GetRequest{Id: GetVersionId(cfg)})
            Ω(err).ToNot(HaveOccurred())
            Ω(response).ToNot(BeNil())
            Ω(response).Should(Respect(cfg))
        })
        It("should return an empty configuration for an invalid ID", func() {
            _, err := client.GetConfiguration(context.Background(), &api.GetRequest{Id: "fake"})
            Ω(err).To(HaveOccurred())
        })

        It("should store a valid FSM", func() {
            Ω(store.PutConfig(cfg)).To(Succeed())
            resp, err := client.PutFiniteStateMachine(context.Background(), fsm)
            Ω(err).ToNot(HaveOccurred())
            Ω(resp).ToNot(BeNil())
            Ω(resp.Id).ToNot(BeNil())
            Ω(resp.Fsm).Should(Respect(fsm))
        })
        It("should fail with an invalid Config ID", func() {
            invalid := &api.FiniteStateMachine{ConfigId: "fake"}
            _, err := client.PutFiniteStateMachine(context.Background(), invalid)
            Ω(err).Should(HaveOccurred())
        })
        It("can retrieve a stored FSM", func() {
            id := "123456"
            Ω(store.PutConfig(cfg))
            Ω(store.PutStateMachine(id, fsm)).Should(Succeed())
            Ω(client.GetFiniteStateMachine(context.Background(), &api.GetRequest{Id: id})).Should(
                Respect(fsm))
        })
        It("will return an error for an invalid ID", func() {
            _, err := client.GetFiniteStateMachine(context.Background(), &api.GetRequest{Id: "fake"})
            Ω(err).Should(HaveOccurred())
        })
    })
})
