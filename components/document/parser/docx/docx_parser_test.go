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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components/document/parser"
)

func TestDocxParser_Parse(t *testing.T) {
	t.Run("DocxParser_Parse_with_section", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test_docx.docx")
		assert.NoError(t, err)

		p, err := NewDocxParser(ctx, &Config{
			ToSections:     true,
			IncludeHeaders: true,
			IncludeFooters: true,
			IncludeTables:  true,
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.Equal(t, 4, len(docs))
		for _, doc := range docs {
			typ, _ := GetSectionType(doc)
			assert.Equal(t, typ, doc.MetaData[SectionTypeKey])
		}

	})

	t.Run("DocxParser_Parse_without_section", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test_docx.docx")
		assert.NoError(t, err)

		p, err := NewDocxParser(ctx, &Config{
			ToSections:     false,
			IncludeHeaders: true,
			IncludeFooters: true,
			IncludeTables:  true,
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(docs))
		typ, _ := GetSectionType(docs[0])
		assert.Equal(t, typ, docs[0].MetaData[SectionTypeKey])

	})
}
