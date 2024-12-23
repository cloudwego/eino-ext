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

package pdf

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/stretchr/testify/assert"
)

func TestLoader_Load(t *testing.T) {
	t.Run("TestLoader_Load", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./testdata/test_pdf.pdf")
		assert.NoError(t, err)

		p, err := NewPDFParser(ctx, nil)
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, WithToPages(true), parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(docs))
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[0].MetaData)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"test": "test"}, docs[1].MetaData)
	})
}
