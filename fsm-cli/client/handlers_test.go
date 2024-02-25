package client_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

)

var _ = Describe("Handlers", func() {
	Context("sending YAML objects", func() {
		dataFile := "../data/config.yaml"
		It("should be able to send files", func() {
			Expect(svc.Send(dataFile)).To(Succeed())
		})
	})
})
