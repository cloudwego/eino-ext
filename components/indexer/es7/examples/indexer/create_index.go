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

package main

import (
	"context"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
)

// createIndex create index for example in add_documents.go.
// ES7 uses untyped API, so we construct the mapping as JSON.
func createIndex(ctx context.Context, client *elasticsearch.Client) error {
	mapping := `{
		"mappings": {
			"properties": {
				"content": {
					"type": "text"
				},
				"location": {
					"type": "keyword"
				},
				"content_vector": {
					"type": "dense_vector",
					"dims": 1024
				}
			}
		}
	}`

	res, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithContext(ctx),
		client.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil // Index may already exist
	}

	return nil
}
