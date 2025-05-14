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
	t.Run("TestXlsxParser_Default", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./testdata/location.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, nil)
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{}, docs[0].MetaData[PrefixRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[PrefixExt])
	})

	t.Run("TestXlsxParser_WithAnotherSheet", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./testdata/location.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet2",
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{}, docs[0].MetaData[PrefixRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[PrefixExt])
	})

	t.Run("TestXlsxParser_WithHeader", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./testdata/location.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, &Config{
			SheetName: "Sheet3",
			HasHeader: true,
		})
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"县（区）": "新密市", "市": "郑州市", "省": "河南省"}, docs[0].MetaData[PrefixRow])
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData[PrefixExt])
	})
}
