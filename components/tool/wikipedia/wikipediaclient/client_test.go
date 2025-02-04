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

package wikipediaclient

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient(
		WithLanguage("en"),
		WithUserAgent("eino (https://github.com/cloudwego/eino)"),
		WithTopK(3),
		WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
	)
	assert.NotNil(t, c)
}

func TestSearchAndGetPage(t *testing.T) {
	c := NewClient(
		WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
		WithLanguage("en"),
		WithUserAgent("eino (https://github.com/cloudwego/eino)"),
		WithTopK(3),
	)

	results, err := c.Search(context.Background(), "bytedance")
	assert.NoError(t, err)
	assert.NotNil(t, results)

	results, err = c.Search(context.Background(), "")
	assert.Error(t, err, ErrInvalidParameters)
	assert.Nil(t, results)

	for _, result := range results {
		pr, err := c.GetPage(context.Background(), result.Title)
		assert.NoError(t, err)
		assert.NotNil(t, pr)
	}

	pr, err := c.GetPage(context.Background(), "xxxxxxxxx")
	assert.Error(t, err, ErrPageNotFound)
	assert.Nil(t, pr)

}
