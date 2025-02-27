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

package bingcore

import (
	"fmt"

	"github.com/bytedance/sonic"
)

// bingAnswer represents the response from Bing search API.
func parseSearchResponse(body []byte) ([]*searchResult, error) {
	var response bingAnswer

	// Unmarshal response body
	err := sonic.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert response to search results
	results := make([]*searchResult, 0, len(response.WebPages.Value))
	for _, resp := range response.WebPages.Value {
		results = append(results, &searchResult{
			Title:       resp.Name,
			URL:         resp.URL,
			Description: resp.Snippet,
		})
	}
	return results, nil
}
