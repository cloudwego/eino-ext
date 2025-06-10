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

package xlsx

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/stretchr/testify/assert"
)

func TestXlsxParser_Parse(t *testing.T) {
	t.Run("TestXlsxParser_WithDefault", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, nil)

		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"年龄": "21", "性别": "男", "姓名": "张三"}, docs[0].MetaData[MetaDataRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[MetaDataExt])
	})

	t.Run("TestXlsxParser_WithAnotherSheet", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"年龄": "21", "性别": "男", "姓名": "张三"}, docs[0].MetaData[MetaDataRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[MetaDataExt])
	})

	t.Run("TestXlsxParser_WithIDPrefix", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
			IDPrefix:  "_xlsx_row_",
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"年龄": "21", "性别": "男", "姓名": "张三"}, docs[0].MetaData[MetaDataRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[MetaDataExt])
	})

	t.Run("TestXlsxParser_WithNoHeader", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet3",
			NoHeader:  true,
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{}, docs[0].MetaData[MetaDataRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[MetaDataExt])
	})
}
