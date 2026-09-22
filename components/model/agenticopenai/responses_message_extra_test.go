/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMarkAutoCachedForCallerLeavesPublishedExtraUnchanged(t *testing.T) {
	original := &schema.AgenticMessage{Extra: map[string]any{"keep": "yes"}}

	stop := make(chan struct{})
	sawMark := make(chan struct{}, 1)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			for key, value := range original.Extra {
				if key == keyOfResponseAutoCached {
					select {
					case sawMark <- struct{}{}:
					default:
					}
					return
				}
				_ = value
			}
		}
	}()

	for i := 0; i < 200; i++ {
		got := markAutoCachedForCaller(original)
		if got == original {
			t.Fatal("expected a cloned message")
		}
		if got.Extra["keep"] != "yes" {
			t.Fatalf("clone dropped existing Extra entry: %#v", got.Extra)
		}
		if got.Extra[keyOfResponseAutoCached] != true {
			t.Fatalf("clone missing auto-cache mark: %#v", got.Extra)
		}
	}
	close(stop)
	<-readDone

	select {
	case <-sawMark:
		t.Fatalf("published Extra contains %s", keyOfResponseAutoCached)
	default:
	}
	if _, ok := original.Extra[keyOfResponseAutoCached]; ok {
		t.Fatalf("published Extra was written: %#v", original.Extra)
	}
}

func TestMarkAutoCachedForCallerNilExtra(t *testing.T) {
	original := &schema.AgenticMessage{}
	got := markAutoCachedForCaller(original)
	if got == original {
		t.Fatal("expected a cloned message")
	}
	if original.Extra != nil {
		t.Fatalf("nil Extra on the published message became %#v", original.Extra)
	}
	if got.Extra[keyOfResponseAutoCached] != true {
		t.Fatalf("clone missing auto-cache mark: %#v", got.Extra)
	}
}

func TestMarkAutoCachedForCallerNilMessage(t *testing.T) {
	if got := markAutoCachedForCaller(nil); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}
