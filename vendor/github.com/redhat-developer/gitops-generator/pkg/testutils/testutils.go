//
// Copyright 2021-2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils

import (
	"regexp"
	"sync"
	"testing"
)

type Execution struct {
	BaseDir string
	Command string
	Args    []string
}

type ErrorStack struct {
	Errors []error
	sync.Mutex
}

func NewErrors() *ErrorStack {
	return &ErrorStack{
		Errors: []error{},
	}
}

func (s *ErrorStack) Push(err error) {
	s.Lock()
	defer s.Unlock()
	s.Errors = append(s.Errors, err)
}

func (s *ErrorStack) Pop() error {
	s.Lock()
	defer s.Unlock()
	if len(s.Errors) == 0 {
		return nil
	}
	err := s.Errors[len(s.Errors)-1]
	s.Errors = s.Errors[0 : len(s.Errors)-1]
	return err
}

type OutputStack struct {
	Outputs [][]byte
	sync.Mutex
}

func NewOutputs(o ...[]byte) *OutputStack {
	return &OutputStack{
		Outputs: o,
	}
}

func (s *OutputStack) Pop() []byte {
	s.Lock()
	defer s.Unlock()
	if len(s.Outputs) == 0 {
		return []byte("")
	}
	o := s.Outputs[len(s.Outputs)-1]
	s.Outputs = s.Outputs[0 : len(s.Outputs)-1]
	return o
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func AssertErrorMatch(t *testing.T, msg string, testErr error) {
	t.Helper()
	if !ErrorMatch(t, msg, testErr) {
		t.Fatalf("failed to match error: '%s' did not match %v", testErr, msg)
	}
}

// ErrorMatch returns true if an error matches the required string.
//
// e.g. ErrorMatch(t, "failed to open", err) would return true if the
// err passed in had a string that matched.
//
// The message can be a regular expression, and if this fails to compile, then
// the test will fail.
func ErrorMatch(t *testing.T, msg string, testErr error) bool {
	t.Helper()
	if msg == "" && testErr == nil {
		return true
	}
	if msg != "" && testErr == nil {
		return false
	}
	match, err := regexp.MatchString(msg, testErr.Error())
	if err != nil {
		t.Fatal(err)
	}
	return match
}
