package client_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Insecure (no TLS) CLI tests: focus on a couple of basic endpoints.
var _ = Describe("Handlers (insecure)", func() {
	Context("basic gRPC operations without TLS", func() {
		It("should report READY state via Health", func() {
			resp, err := insecureClient.Health(context.Background(), &emptypb.Empty{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.GetState().String()).To(Equal("READY"))
		})

		It("should be able to call GetAllConfigurations", func() {
			// Using an empty filter returns all configuration names; this should
			// succeed whether or not any configurations are present.
			resp, err := insecureClient.GetAllConfigurations(
				context.Background(), &wrapperspb.StringValue{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
		})
	})
})

// TLS-enabled CLI tests: exercise the full send pipeline and Health.
var _ = Describe("Handlers (TLS)", func() {
	Context("sending YAML objects over TLS", func() {
		It("should be able to send a Configuration", func() {
			dataFile := "../../data/config.yaml"
			Expect(tlsClient.Send(dataFile)).To(Succeed())
		})

		It("should be able to send an FSM definition", func() {
			// Precondition: configuration has been created in a previous test.
			Expect(tlsClient.Send("../../data/order.yaml")).To(Succeed())
		})

		It("should be able to send an Event and receive an outcome", func() {
			// Precondition: configuration and FSM have been created by previous tests.
			Expect(tlsClient.Send("../../data/evt.yaml")).To(Succeed())
		})
	})

	Context("basic gRPC health over TLS", func() {
		It("should report READY state", func() {
			resp, err := tlsClient.Health(context.Background(), &emptypb.Empty{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.GetState().String()).To(Equal("READY"))
		})
	})
})
