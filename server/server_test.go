/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
