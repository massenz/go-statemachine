package server_test

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/massenz/go-statemachine/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/massenz/go-statemachine/api"

	"github.com/massenz/go-statemachine/server"
)

var _ = Describe("Handlers", func() {
	var (
		req     *http.Request
		handler http.Handler
		writer  *httptest.ResponseRecorder
	)
	Context("when creating configurations", func() {
		var store storage.StoreManager
		BeforeEach(func() {
			handler = http.HandlerFunc(server.CreateConfigurationHandler)
			writer = httptest.NewRecorder()
			store = storage.NewInMemoryStore()
			server.SetStore(store)
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
				// TODO: decode JSON and confirm the ID is valid, etc...
			})
			It("should fill the cache", func() {
				handler.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				_, found := store.GetConfig("test.orders:v1")
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
	Context("when retrieving configurations", func() {
		var router *mux.Router
		var store storage.StoreManager
		BeforeEach(func() {
			handler = http.HandlerFunc(server.GetConfigurationHandler)
			writer = httptest.NewRecorder()
			router = server.NewRouter()
			store = storage.NewInMemoryStore()
			server.SetStore(store)
		})
		Context("with a valid cfg_id", func() {
			var spaceship = api.Configuration{
				Name:          "spaceship",
				Version:       "v1",
				StartingState: "earth",
				States:        []string{"earth", "orbit", "mars"},
				Transitions: []*api.Transition{
					{From: "earth", To: "orbit", Event: "launch"},
					{From: "orbit", To: "mars", Event: "land"},
				},
			}
			var cfgId = spaceship.GetVersionId()
			BeforeEach(func() {
				endpoint := server.ConfigurationsEndpoint + "/" + cfgId
				req = httptest.NewRequest(http.MethodGet, endpoint, nil)
				Expect(store.PutConfig(cfgId, &spaceship)).ToNot(HaveOccurred())
			})

			It("should succeed", func() {
				var result api.Configuration
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusOK))
				Expect(json.NewDecoder(writer.Body).Decode(&result)).ToNot(HaveOccurred())
				Expect(result.GetVersionId()).To(Equal(cfgId))
				Expect(result.States).To(Equal(spaceship.States))
				Expect(len(result.Transitions)).To(Equal(len(spaceship.Transitions)))
				for n, t := range result.Transitions {
					Expect(t.From).To(Equal(spaceship.Transitions[n].From))
					Expect(t.To).To(Equal(spaceship.Transitions[n].To))
					Expect(t.Event).To(Equal(spaceship.Transitions[n].Event))
				}
			})
		})

		Context("with an invalid Id", func() {
			BeforeEach(func() {
				endpoint := server.ConfigurationsEndpoint + "/fake:v3"
				req = httptest.NewRequest(http.MethodGet, endpoint, nil)
			})
			It("will return Not Found", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusNotFound))
			})
		})
		Context("without Id", func() {
			BeforeEach(func() {
				endpoint := server.ConfigurationsEndpoint
				req = httptest.NewRequest(http.MethodGet, endpoint, nil)
			})
			It("it will fail", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusMethodNotAllowed))
			})
		})
	})
})
