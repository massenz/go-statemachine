package server_test

import (
	"bytes"
	"encoding/json"
	"github.com/massenz/go-statemachine/storage"
	log "github.com/massenz/slf4go/logging"
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

var _ = Describe("Configuration Handlers", func() {
	var (
		req    *http.Request
		writer *httptest.ResponseRecorder
		store  storage.StoreManager

		// NOTE: we are using the Router here as we need to correctly also parse
		// the URI for path args (just using the handler will not do that)
		// The `router` can be safely set for all the test contexts, once and for all.
		router = server.NewRouter()
	)
	// Disabling verbose logging, as it pollutes test output;
	// set it back to DEBUG when tests fail, and you need to
	// diagnose the failure.
	server.SetLogLevel(log.WARN)

	Context("when creating configurations", func() {
		BeforeEach(func() {
			writer = httptest.NewRecorder()
			store = storage.NewInMemoryStore()
			server.SetStore(store)
		})
		Context("with a valid JSON", func() {
			BeforeEach(func() {
				configJson, err := ioutil.ReadFile("../data/orders.json")
				Expect(err).ToNot(HaveOccurred())
				body := bytes.NewReader(configJson)
				req = httptest.NewRequest(http.MethodPost, server.ConfigurationsEndpoint, body)
			})

			It("should succeed", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				location := writer.Header().Get("Location")
				Expect(strings.HasSuffix(location, "/test.orders:v1")).To(BeTrue())

				response := api.Configuration{}
				Expect(json.Unmarshal(writer.Body.Bytes(), &response)).ToNot(HaveOccurred())
				Expect(response.Name).To(Equal("test.orders"))
				Expect(response.States).To(Equal([]string{
					"start",
					"pending",
					"shipping",
					"delivered",
					"complete",
					"closed",
				}))
				Expect(response.StartingState).To(Equal("start"))
			})
			It("should fill the cache", func() {
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusCreated))
				_, found := store.GetConfig("test.orders:v1")
				Expect(found).To(BeTrue())
			})
		})

		Context("with an invalid JSON", func() {
			var body io.Reader
			BeforeEach(func() {
				req = httptest.NewRequest(http.MethodPost, server.ConfigurationsEndpoint, body)
			})
			It("without name, states or transitions, will fail", func() {
				body = strings.NewReader(`{
					"version": "v1",
					"starting_state": "source"
				}`)
				router.ServeHTTP(writer, req)
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
				router.ServeHTTP(writer, req)
				Expect(writer.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
	Context("when retrieving configurations", func() {
		BeforeEach(func() {
			writer = httptest.NewRecorder()
			// We need an empty, clean store for each test to avoid cross-polluting it.
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
				Expect(store.PutConfig(cfgId, &spaceship)).ToNot(HaveOccurred())
				endpoint := strings.Join([]string{server.ConfigurationsEndpoint, cfgId}, "/")
				req = httptest.NewRequest(http.MethodGet, endpoint, nil)
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
