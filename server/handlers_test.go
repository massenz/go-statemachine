package server_test

import (
	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/massenz/go-statemachine/server"
)

var _ = Describe("Handlers", func() {
	var (
		req     *http.Request
		handler http.Handler
		writer  *httptest.ResponseRecorder
	)
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

		Context("with an invalid JSON", func() {
			var body io.Reader
			BeforeEach(func() {
				req = httptest.NewRequest(http.MethodGet, server.ConfigurationsEndpoint, body)

			})
			It("without name, states or transitions, will fail", func() {
				body = strings.NewReader(`{
					"version": "v1",
					"starting_state": "source"
				}`)
				handler.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusBadRequest))
			})
			It("without states, will fail", func() {
				body = strings.NewReader(`{
					"name": "fake",
					"version": "v1",
					"starting_state": "source"
					"transitions": [
						{"from": "source", "to": "tested", "event": "test"},
						{"from": "tested", "to": "binary", "event": "build"}
					],
				}`)
				handler.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
})
