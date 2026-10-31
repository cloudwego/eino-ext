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

package browseruse

import (
	"context"
	"strings"
	"testing"
)

func TestInvokableRunRejectsNegativeClickIndex(t *testing.T) {
	b := &Tool{elements: []ElementInfo{{Index: 0}}}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("negative click index panicked: %v", p)
		}
	}()
	result, err := b.InvokableRun(context.Background(), `{"action":"click_element","index":-1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "out of range") {
		t.Fatalf("expected out-of-range tool error, got %s", result)
	}
}
