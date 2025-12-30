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
	"fmt"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/stretchr/testify/assert"
)

func TestNewRetriever(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieve_documents", func(t *testing.T) {
		r, err := NewRetriever(ctx, &RetrieverConfig{
			Client: &elasticsearch.Client{},
			Index:  "eino_ut",
			TopK:   10,
			ResultParser: func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error) {
				var mp map[string]any
				if err := json.Unmarshal(hit.Source_, &mp); err != nil {
					return nil, err
				}

				var id string
				if hit.Id_ != nil {
					id = *hit.Id_
				}

				content, ok := mp["eino_doc_content"].(string)
				if !ok {
					return nil, fmt.Errorf("content not found")
				}

				return &schema.Document{
					ID:       id,
					Content:  content,
					MetaData: nil,
				}, nil
			},
			SearchMode: &mockSearchMode{},
		})
		assert.NoError(t, err)

		mockSearch := search.NewSearchFunc(r.client)()

		defer mockey.Mock(mockey.GetMethod(mockSearch, "Index")).
			Return(mockSearch).Build().Patch().UnPatch()

		defer mockey.Mock(mockey.GetMethod(mockSearch, "Request")).
			Return(mockSearch).Build().Patch().UnPatch()

		defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
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

	t.Run("default_result_parser", func(t *testing.T) {
		r, err := NewRetriever(ctx, &RetrieverConfig{
			Client: &elasticsearch.Client{},
			Index:  "eino_ut",
			TopK:   10,
			// No ResultParser provided, should use default
			SearchMode: &mockSearchMode{},
		})
		assert.NoError(t, err)

		mockSearch := search.NewSearchFunc(r.client)()

		defer mockey.Mock(mockey.GetMethod(mockSearch, "Index")).
			Return(mockSearch).Build().Patch().UnPatch()

		defer mockey.Mock(mockey.GetMethod(mockSearch, "Request")).
			Return(mockSearch).Build().Patch().UnPatch()

		t.Run("success", func(t *testing.T) {
			defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
				Hits: types.HitsMetadata{
					Hits: []types.Hit{
						{
							Id_:    func() *string { s := "doc_1"; return &s }(),
							Score_: func() *types.Float64 { f := types.Float64(0.9); return &f }(),
							Source_: json.RawMessage([]byte(`{
						  "content": "default parser content",
						  "extra": "metadata"
						}`)),
						},
					},
				},
			}, nil).Build().Patch().UnPatch()

			docs, err := r.Retrieve(ctx, "test query")
			assert.NoError(t, err)

			assert.Len(t, docs, 1)
			assert.Equal(t, "doc_1", docs[0].ID)
			assert.Equal(t, "default parser content", docs[0].Content)
			assert.Equal(t, "metadata", docs[0].MetaData["extra"])
			assert.Equal(t, 0.9, docs[0].Score())
		})

		t.Run("missing_id", func(t *testing.T) {
			defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
				Hits: types.HitsMetadata{
					Hits: []types.Hit{
						{
							Score_: func() *types.Float64 { f := types.Float64(0.9); return &f }(),
							Source_: json.RawMessage([]byte(`{
						  "content": "default parser content"
						}`)),
						},
					},
				},
			}, nil).Build().Patch().UnPatch()

			_, err := r.Retrieve(ctx, "test query")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "field '_id' not found")
		})

		t.Run("missing_source", func(t *testing.T) {
			defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
				Hits: types.HitsMetadata{
					Hits: []types.Hit{
						{
							Id_: func() *string { s := "doc_1"; return &s }(),
						},
					},
				},
			}, nil).Build().Patch().UnPatch()

			_, err := r.Retrieve(ctx, "test query")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "field '_source' not found")
		})

		t.Run("missing_content", func(t *testing.T) {
			defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
				Hits: types.HitsMetadata{
					Hits: []types.Hit{
						{
							Id_: func() *string { s := "doc_1"; return &s }(),
							Source_: json.RawMessage([]byte(`{
						  "other": "field"
						}`)),
						},
					},
				},
			}, nil).Build().Patch().UnPatch()

			_, err := r.Retrieve(ctx, "test query")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "field 'content' not found")
		})

		t.Run("invalid_content_type", func(t *testing.T) {
			defer mockey.Mock(mockey.GetMethod(mockSearch, "Do")).Return(&search.Response{
				Hits: types.HitsMetadata{
					Hits: []types.Hit{
						{
							Id_: func() *string { s := "doc_1"; return &s }(),
							Source_: json.RawMessage([]byte(`{
						  "content": 123
						}`)),
						},
					},
				},
			}, nil).Build().Patch().UnPatch()

			_, err := r.Retrieve(ctx, "test query")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "field 'content' in document doc_1 is not a string")
		})
	})
}

type mockSearchMode struct{}

func (m *mockSearchMode) BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error) {
	return &search.Request{}, nil
}
