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
			Columns: Columns{
				NoHeader: true,
			},
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{}, docs[0].MetaData[MetaDataRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[MetaDataExt])
	})

	t.Run("TestXlsxParser_WithColumns_Content", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
			Columns: Columns{
				Content: []string{"A"}, // Only use column A for content
			},
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f)
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)

		// Assuming column A contains "张三" in the first row
		assert.Equal(t, "张三", docs[0].Content)

		// Metadata should still be populated from all columns
		assert.Equal(t, map[string]any{"年龄": "21", "性别": "男", "姓名": "张三"}, docs[0].MetaData[MetaDataRow])
	})

	t.Run("TestXlsxParser_WithColumns_Meta", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
			Columns: Columns{
				Meta: []string{"A", "C"}, // Only use columns A and C for metadata
			},
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f)
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)

		// Content should include all columns
		assert.True(t, len(docs[0].Content) > 0)

		// Metadata should only include columns A and C
		metaData := docs[0].MetaData[MetaDataRow].(map[string]any)
		assert.Equal(t, 2, len(metaData))
		assert.Contains(t, metaData, "姓名")
		assert.Contains(t, metaData, "年龄")
		assert.NotContains(t, metaData, "性别")
	})

	t.Run("TestXlsxParser_WithColumns_CustomNames", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
			Columns: Columns{
				CustomNames: map[string]string{
					"A": "Name",
					"B": "Gender",
					"C": "Age",
				},
			},
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f)
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)

		// Content should include all columns
		assert.True(t, len(docs[0].Content) > 0)

		// Metadata should use custom names
		metaData := docs[0].MetaData[MetaDataRow].(map[string]any)
		assert.Equal(t, map[string]any{
			"Name":   "张三",
			"Gender": "男",
			"Age":    "21",
		}, metaData)
	})

	t.Run("TestXlsxParser_WithColumns_Combined", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
			Columns: Columns{
				Content: []string{"A"},
				Meta:    []string{"B", "C"},
				CustomNames: map[string]string{
					"B": "Gender",
					"C": "Age",
				},
			},
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f)
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)

		// Content should only include column A
		assert.Equal(t, "张三", docs[0].Content)

		// Metadata should only include columns B and C with custom names
		metaData := docs[0].MetaData[MetaDataRow].(map[string]any)
		assert.Equal(t, map[string]any{
			"Gender": "男",
			"Age":    "21",
		}, metaData)
	})

	t.Run("TestXlsxParser_ConfigFromOptions", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./examples/testdata/test.xlsx")
		assert.NoError(t, err)

		// Create parser with default config
		p, err := NewXlsxParser(ctx, nil)
		assert.NoError(t, err)

		// Override config through options
		optionConfig := &Config{
			SheetName: "Sheet2",
			Columns: Columns{
				Content: []string{"A"},
				CustomNames: map[string]string{
					"A": "Name",
				},
			},
		}

		docs, err := p.Parse(ctx, f, WithConfig(optionConfig))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)

		// Content should only include column A
		assert.Equal(t, "张三", docs[0].Content)

		// Metadata should use custom name for column A
		metaData := docs[0].MetaData[MetaDataRow].(map[string]any)
		assert.Contains(t, metaData, "Name")
		assert.Equal(t, "张三", metaData["Name"])
	})
}
