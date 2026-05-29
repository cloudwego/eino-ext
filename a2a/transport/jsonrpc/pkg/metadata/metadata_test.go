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

package metadata

import (
	"context"
	"testing"
)

func TestMetadataValues(t *testing.T) {
	ctx := context.Background()
	_, ok := GetValue(ctx, "k")
	if ok {
		t.Fatalf("expected empty metadata")
	}
	ctx = WithValue(ctx, "k", "v")
	if val, ok := GetValue(ctx, "k"); !ok || val != "v" {
		t.Fatalf("unexpected value: %v %v", val, ok)
	}
	all, ok := GetAllValues(ctx)
	if !ok || all["k"] != "v" {
		t.Fatalf("unexpected all values")
	}
	ctx = WithValue(ctx, "k", "v2")
	if val, ok := GetValue(ctx, "k"); !ok || val != "v2" {
		t.Fatalf("unexpected value: %v %v", val, ok)
	}
	ctx = ClearValue(ctx)
	if _, ok := GetValue(ctx, "k"); ok {
		t.Fatalf("expected cleared metadata")
	}
}