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

package statemachine_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	sm "github.com/massenz/go-statemachine/statemachine"
)

var _ = Describe("Finite State Machines", func() {

	var simple = sm.Configuration{
		StartingState: "one",
		States:        []sm.State{"one", "two", "three"},
		Transitions: []sm.Transition{
			{"one", "two", "go"},
			{"two", "three", "land"},
		},
	}

	Context("if well defined", func() {
		fsm := sm.NewFSM(&simple)
		It("should be created without errors", func() {
			Expect(fsm).ShouldNot(BeNil())
			Expect(fsm.State()).Should(Equal(sm.State("one")))

		})
		It("should allow valid transitions", func() {
			Expect(fsm.SendEvent("go")).ShouldNot(HaveOccurred())
			Expect(fsm.State()).To(Equal(sm.State("two")))

			Expect(fsm.SendEvent("land")).ShouldNot(HaveOccurred())
			Expect(fsm.State()).To(Equal(sm.State("three")))

		})

		It("should return an error for an invalid transition", func() {
			Expect(fsm.SendEvent("foo")).To(HaveOccurred())
		})

		It("can be reset", func() {
			fsm.Reset()
			Expect(fsm.State()).To(Equal(sm.State("one")))
		})
	})
})
