package storage_test

import (
	"github.com/massenz/go-statemachine/pkg/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {

	It("should correctly match a regex", func() {
		res := storage.IsNotFoundErr(storage.NotFoundError("fsm:test#fake-fsm"))
		Expect(res).To(BeTrue())
	})
	It("should not match the wrong regex", func() {
		res := storage.IsNotFoundErr(storage.AlreadyExistsError("fsm:test#fake-fsm"))
		Expect(res).ToNot(BeTrue())
	})
})
