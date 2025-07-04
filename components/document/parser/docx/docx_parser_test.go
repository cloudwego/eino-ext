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

package docx

import (
	"context"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestDocxParser_Parse(t *testing.T) {
	t.Run("DocxParser_Parse", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test_docx.docx")
		assert.NoError(t, err)

		p, err := NewDocxParser(ctx, &Config{
			ToSections:      true,
			IncludeComments: true,
			IncludeHeaders:  true,
			IncludeFooters:  true,
			IncludeTables:   true,
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.Equal(t, 5, len(docs))
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData)
		assert.True(t, len(docs[1].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[1].MetaData)
		assert.True(t, len(docs[2].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[2].MetaData)
		assert.True(t, len(docs[3].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[3].MetaData)
		assert.True(t, len(docs[4].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[4].MetaData)
	})
}
