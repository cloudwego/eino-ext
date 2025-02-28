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

package knowledge

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func TestRetrieve(t *testing.T) {
	ctx := context.Background()
	conf := &Config{
		BaseURL:   "api-knowledgebase.mlp.cn-beijing.volces.com",
		AK:        "test-ak",
		SK:        "test-sk",
		AccountID: "test-account-id",
		Name:      "test-name",
		Limit:     10,
	}

	retriever, err := NewRetriever(ctx, conf)
	assert.NoError(t, err)

	mockey.PatchConvey("Test Retrieve", t, func() {
		mockey.Mock((*http.Client).Do).To(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, req.URL.Path, path)
			respBody := `{
				"code": 0,
				"data": {
					"result_list": [
						{
							"id": "doc1",
							"content": "This is a test document."
						}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(respBody)),
			}, nil
		}).Build()

		docs, err := retriever.Retrieve(ctx, "test query")
		assert.NoError(t, err)
		assert.Len(t, docs, 1)
		assert.Equal(t, "doc1", docs[0].ID)
		assert.Equal(t, "This is a test document.", docs[0].Content)
	})
}
