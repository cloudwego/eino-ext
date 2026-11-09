/*
 * Copyright 2026 CloudWeGo Authors
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
 */

package langfuse

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// Mirrors adk.reactRunInput: all state in unexported fields, one pointing at an
// exported struct. The gate is keyed by package path and type name and Eino's
// real type cannot be declared here, so tests point it at this stand-in; that the
// real name matches is covered by TestHandlerExportsUnexportedAgentRunInput.
type unexportedRunInput struct {
	input       *unexportedRunInputPayload
	instruction string
}

type unexportedRunInputPayload struct {
	Messages []string
	Stream   bool
}

// Same shape as above, deliberately never listed, to pin the gate.
type unlistedRunInput struct {
	input       *unexportedRunInputPayload
	instruction string
}

// An unencodable unexported field must not take the whole payload down.
type unexportedRunInputWithFunc struct {
	fn   func()
	name string
}

// time.Time inside an unexported field must keep its MarshalJSON form.
type unexportedRunInputWithTime struct {
	at   time.Time
	name string
}

// Custom MarshalJSON returning {} must never be overwritten.
type unexportedRunInputOpaque struct {
	Visible string
}

func (unexportedRunInputOpaque) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// listRunInputType points the gate at the exact type of value.
func listRunInputType(t *testing.T, value any) {
	t.Helper()
	rt := reflect.TypeOf(value)
	saved := einoRunInputTypes
	einoRunInputTypes = map[string]bool{rt.PkgPath() + "." + rt.Name(): true}
	t.Cleanup(func() { einoRunInputTypes = saved })
}

func TestMarshalAttribute(t *testing.T) {
	cases := []struct {
		name     string
		listed   bool
		in       any
		standard string
		want     string
	}{
		{
			name:     "listed run input: rebuilt from its unexported fields",
			listed:   true,
			in:       unexportedRunInput{input: &unexportedRunInputPayload{Messages: []string{"hi"}, Stream: true}, instruction: "you are helpful"},
			standard: "{}",
			want:     `{"input":{"Messages":["hi"],"Stream":true},"instruction":"you are helpful"}`,
		},
		{
			name:     "listed run input holding an unencodable field: drop that field, keep the rest",
			listed:   true,
			in:       unexportedRunInputWithFunc{fn: func() {}, name: "n"},
			standard: "{}",
			want:     `{"name":"n"}`,
		},
		{
			name:     "listed run input holding a time.Time: keep the RFC 3339 form",
			listed:   true,
			in:       unexportedRunInputWithTime{at: time.Date(2026, 9, 26, 14, 0, 0, 0, time.UTC), name: "n"},
			standard: "{}",
			want:     `{"at":"2026-09-26T14:00:00Z","name":"n"}`,
		},
		{
			name:     "same shape but not listed: untouched",
			in:       unlistedRunInput{input: &unexportedRunInputPayload{Messages: []string{"hi"}}, instruction: "i"},
			standard: "{}",
			want:     "{}",
		},
		{
			name:     "custom MarshalJSON returning {}: untouched",
			in:       unexportedRunInputOpaque{Visible: "x"},
			standard: "{}",
			want:     "{}",
		},
		{
			name:     "nil pointer stays null",
			in:       (*unexportedRunInput)(nil),
			standard: "null",
			want:     "null",
		},
		{
			name:     "non struct values are untouched",
			in:       42,
			standard: "42",
			want:     "42",
		},
		{
			name:     "empty struct stays an empty object",
			in:       struct{}{},
			standard: "{}",
			want:     "{}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.listed {
				listRunInputType(t, tc.in)
			}

			std, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("standard marshal failed: %v", err)
			}
			if string(std) != tc.standard {
				t.Fatalf("premise broken: standard output is %s, want %s", std, tc.standard)
			}

			got, err := marshalAttribute(tc.in)
			if err != nil {
				t.Fatalf("marshalAttribute failed: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("unexpected output\n  got:  %s\n  want: %s", got, tc.want)
			}
		})
	}
}

// TestMarshalAttribute_NoOpWhenStandardEncoderWorks is the regression anchor for
// every value that already works: whenever json.Marshal does not return {}, the
// output must be byte for byte identical.
func TestMarshalAttribute_NoOpWhenStandardEncoderWorks(t *testing.T) {
	inputs := []any{
		42, "s", true, 3.14, nil,
		[]int{1, 2},
		map[string]int{"a": 1},
		unexportedRunInputPayload{Messages: []string{"m"}},
		time.Date(2026, 9, 26, 14, 0, 0, 0, time.UTC),
	}
	for _, in := range inputs {
		std, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("standard marshal failed: %v", err)
		}
		got, err := marshalAttribute(in)
		if err != nil {
			t.Fatalf("marshalAttribute failed: %v", err)
		}
		if string(got) != string(std) {
			t.Errorf("input %#v was changed:\n  standard: %s\n  got:      %s", in, std, got)
		}
	}
}
