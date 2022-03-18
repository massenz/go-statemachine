package server_test

import (
	"bytes"
	"github.com/massenz/go-statemachine/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Server", func() {
	var (
		req     *http.Request
		handler http.Handler
		writer  *httptest.ResponseRecorder
	)
	Context("when started", func() {
		BeforeEach(func() {
			handler = http.HandlerFunc(server.HealthHandler)
			req = httptest.NewRequest(http.MethodGet, "/health", nil)
			writer = httptest.NewRecorder()
		})
		It("is healthy", func() {
			handler.ServeHTTP(writer, req)
			Expect(writer.Code).To(Equal(http.StatusOK))
		})
	})

	Context("when creating configurations", func() {
		BeforeEach(func() {
			handler = http.HandlerFunc(server.CreateConfigurationHandler)
			writer = httptest.NewRecorder()
		})
		Context("with a valid JSON", func() {
			BeforeEach(func() {
				configJson, err := ioutil.ReadFile("../data/orders.json")
				Expect(err).ToNot(HaveOccurred())
				body := bytes.NewReader(configJson)
				req = httptest.NewRequest(http.MethodGet, server.ConfigurationsEndpoint, body)
			})

			It("should succeed", func() {
				handler.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				Expect(writer.Header().Get("Location")).To(Equal(
					server.ConfigurationsEndpoint + "/test.orders:v1"))
				// TODO: decode JSON and confim the ID is valid, etc...
			})
			It("should fill the cache", func() {
				_, found := server.GetConfig("test.orders:v1")
				Expect(found).To(BeTrue())
			})
		})
	})
})
