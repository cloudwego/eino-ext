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

package file

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components/document"
)

func TestFileLoader_Load(t *testing.T) {
	t.Run("TestFileLoader_Load", func(t *testing.T) {
		ctx := context.Background()
		loader, err := NewFileLoader(ctx, &FileLoaderConfig{
			UseNameAsID: true,
		})
		assert.NoError(t, err)

		docs, err := loader.Load(ctx, document.Source{
			URI: "./testdata/test.md",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(docs))
		assert.Equal(t, "test.md", docs[0].ID)
		assert.Equal(t, docs[0].Content, `# Title

- Bullet 1
- Bullet 2`)
		assert.Equal(t, 3, len(docs[0].MetaData))
		assert.Equal(t, "test.md", docs[0].MetaData[MetaKeyFileName])
		assert.Equal(t, ".md", docs[0].MetaData[MetaKeyExtension])
		assert.Equal(t, "./testdata/test.md", docs[0].MetaData[MetaKeySource])
	})
}
