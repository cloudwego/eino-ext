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

import "testing"

func TestUnboundBuffer(t *testing.T) {
	buf := NewUnboundBuffer[int]()
	for i := 0; i < 257; i++ {
		buf.Push(i)
	}
	if first := <-buf.PopChan(); first != 0 {
		t.Fatalf("unexpected first: %d", first)
	}
	buf.Load()
	last := 0
	for i := 0; i < 256; i++ {
		last = <-buf.PopChan()
	}
	if last != 256 {
		t.Fatalf("unexpected last: %d", last)
	}
}