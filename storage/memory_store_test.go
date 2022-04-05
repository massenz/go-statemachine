package storage_test

import (
	"github.com/massenz/go-statemachine/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/massenz/go-statemachine/storage"
)

var _ = Describe("InMemory Store", func() {
	Context("for local testing", func() {
		var store = storage.NewInMemoryStore()

		It("can create an in-memory store", func() {
			Expect(store).ToNot(BeNil())
		})

		Context("can be used to save and retrieve a Configuration", func() {
			var cfg = &api.Configuration{}
			BeforeEach(func() {
				cfg.Name = "my_conf"
				cfg.Version = "v3"
				cfg.StartingState = "start"
				Expect(store.PutConfig(cfg.GetVersionId(), cfg)).ToNot(HaveOccurred())

			})
			It("will give back the saved Configuration", func() {
				found, ok := store.GetConfig(cfg.GetVersionId())
				Expect(ok).To(BeTrue())
				Expect(found).ToNot(BeNil())

				Expect(found.Name).To(Equal(cfg.Name))
				Expect(found.Version).To(Equal(cfg.Version))
				Expect(found.StartingState).To(Equal(cfg.StartingState))

			})

		})

		Context("can be used to save and retrieve a StateMachine", func() {
			var id = "1234"
			var machine *api.FiniteStateMachine

			BeforeEach(func() {
				machine = &api.FiniteStateMachine{
					ConfigId: id,
					State:    "start",
					History:  nil,
				}
				Expect(store.PutStateMachine(id, machine)).ToNot(HaveOccurred())
			})
			It("will give it back unchanged", func() {
				found, ok := store.GetStateMachine(id)
				Expect(ok).To(BeTrue())
				Expect(found).ToNot(BeNil())
				Expect(found.ConfigId).To(Equal(machine.ConfigId))
				Expect(found.History).To(Equal(machine.History))
				Expect(found.State).To(Equal(machine.State))
			})

			It("will return nil for a non-existent id", func() {
				found, ok := store.GetStateMachine("fake")
				Expect(ok).To(BeFalse())
				Expect(found).To(BeNil())
			})
		})

	})
})
