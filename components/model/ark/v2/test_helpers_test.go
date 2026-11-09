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

package ark

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func ptr[T any](v T) *T { return &v }

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func equal(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}

func decodeRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	check(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

func toolInfo(name string) *schema.ToolInfo {
	return &schema.ToolInfo{Name: name, Desc: "inspect a file", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"path": {Type: schema.String, Required: true}})}
}
