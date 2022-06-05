// This code uses the Respect library from https://github.com/JiaYongfei/respect/
// licensed under the MIT License.
// See: https://github.com/JiaYongfei/respect/blob/main/LICENSE

package pubsub_test

import (
	"encoding/json"
	"fmt"
	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/massenz/go-statemachine/pubsub"
)

var _ = Describe("PubSub types", func() {

	Context("when serializing messages", func() {
		var (
			msg    pubsub.EventMessage
			errMsg pubsub.EventErrorMessage
		)
		BeforeEach(func() {
			msg = pubsub.EventMessage{
				Sender:         "test-sender",
				Destination:    "test-dest",
				EventId:        "12345",
				EventName:      "an-event",
				EventTimestamp: time.Now(),
			}
			errMsg = pubsub.EventErrorMessage{
				Error:       *pubsub.NewEventProcessingError(fmt.Errorf("an error")),
				ErrorDetail: "error detail",
				Message:     &msg,
			}

		})
		It("should convert to and from JSON without loss of meaning", func() {
			s := msg.String()
			Expect(s).ToNot(Equal(""))
			var newMsg pubsub.EventMessage
			Expect(json.Unmarshal([]byte(s), &newMsg)).ToNot(HaveOccurred())
			Expect(newMsg).Should(Respect(msg))
		})
		It("should convert errors to and from JSON without loss of meaning", func() {
			s := errMsg.String()
			Expect(s).ToNot(Equal(""))
			var newMsg pubsub.EventErrorMessage
			Expect(json.Unmarshal([]byte(s), &newMsg)).ToNot(HaveOccurred())
			Expect(newMsg).Should(Respect(errMsg))
		})
	})
	Context("when serializing messages with empty fields", func() {
		var msg pubsub.EventMessage
		BeforeEach(func() {
			msg = pubsub.EventMessage{
				EventName: "an-event",
			}
		})

		It("should convert to and from JSON without loss of meaning", func() {
			s := msg.String()
			Expect(s).To(Equal(`{"event_name":"an-event","timestamp":"0001-01-01T00:00:00Z"}`))
			var newMsg pubsub.EventMessage
			Expect(json.Unmarshal([]byte(s), &newMsg)).ToNot(HaveOccurred())
			Expect(newMsg).Should(Respect(msg))
		})
	})
})
