/*
 * Copyright 2024 CloudWeGo Authors
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

package es8

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/stretchr/testify/assert"
)

func TestNewRetriever(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieve_documents", func(t *testing.T) {
		r, err := NewRetriever(ctx, &RetrieverConfig{
			ESConfig:   elasticsearch.Config{},
			Index:      "eino_ut",
			TopK:       10,
			SearchMode: &mockSearchMode{},
		})
		assert.NoError(t, err)

		defer mockey.Mock(mockey.GetMethod(r.client.Search(), "Index")).
			Return(r.client.Search()).Build().Patch().UnPatch()

		defer mockey.Mock(mockey.GetMethod(r.client.Search(), "Request")).
			Return(r.client.Search()).Build().Patch().UnPatch()

		defer mockey.Mock(mockey.GetMethod(r.client.Search(), "Do")).Return(&search.Response{
			Hits: types.HitsMetadata{
				Hits: []types.Hit{
					{
						Source_: json.RawMessage([]byte(`{
  "eino_doc_content": "i'm fine, thank you"
}`)),
					},
				},
			},
		}, nil).Build().Patch().UnPatch()

		docs, err := r.Retrieve(ctx, "how are you")
		assert.NoError(t, err)

		assert.Len(t, docs, 1)
		assert.Equal(t, "i'm fine, thank you", docs[0].Content)
	})

}

type mockSearchMode struct{}

func (m *mockSearchMode) BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error) {
	return &search.Request{}, nil
}
