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

package delete

import (
	"context"
	"fmt"
	"io"
	"net/http"

	sonic "github.com/bytedance/sonic"
)

type DeleteRequest struct {
	URL string `json:"url" jsonschema_description:"The URL to make the DELETE request"`
}

type DeleteResponse struct {
	Content interface{} `json:"content" jsonschema_description:"The response of the DELETE request"`
}

func (r *DeleteRequestTool) Delete(ctx context.Context, req *DeleteRequest) (string, error) {

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, req.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range r.config.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)
	response := DeleteResponse{Content: content}
	jsonResp, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to serialize response: %w", err)
	}

	return string(jsonResp), nil
}
