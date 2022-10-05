/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package server_test

import (
	"github.com/massenz/go-statemachine/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
})
