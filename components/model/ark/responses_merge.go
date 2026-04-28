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

package ark

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func mergeResponsesRequestExtraJSON(req *responses.ResponsesRequest, extra map[string]any) ([]byte, error) {
	if len(extra) == 0 {
		return sonic.Marshal(req)
	}
	baseBytes, err := sonic.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	var base map[string]any
	if err := sonic.Unmarshal(baseBytes, &base); err != nil {
		return nil, fmt.Errorf("unmarshal responses request json: %w", err)
	}
	for k, v := range extra {
		base[k] = v
	}
	out, err := sonic.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("marshal merged request: %w", err)
	}
	return out, nil
}
