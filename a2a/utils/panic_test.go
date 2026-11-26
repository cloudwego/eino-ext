/*
 * Copyright 2025 CloudWeGo Authors
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

package utils

import (
	"strings"
	"testing"
)

func TestNewPanicErr(t *testing.T) {
	err := NewPanicErr("boom", []byte("goroutine 1 [running]:\nmain.foo()\n"))
	if err == nil {
		t.Fatal("nil err")
	}
	msg := err.Error()
	if !strings.Contains(msg, "panic error: boom") {
		t.Errorf("missing info: %q", msg)
	}
	if !strings.Contains(msg, "main.foo()") {
		t.Errorf("missing stack: %q", msg)
	}
}

func TestNewPanicErr_nilStack(t *testing.T) {
	err := NewPanicErr(123, nil)
	if err == nil {
		t.Fatal("nil err")
	}
	if !strings.Contains(err.Error(), "panic error: 123") {
		t.Errorf("info missing: %q", err.Error())
	}
}
