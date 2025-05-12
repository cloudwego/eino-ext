package xlsx

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/stretchr/testify/assert"
)

func TestXlsxParser_Parse(t *testing.T) {
	t.Run("TestXlsxParser_Parse", func(t *testing.T) {
		ctx := context.Background()

		f, err := os.Open("./testdata/location.xlsx")
		assert.NoError(t, err)

		p, err := NewXlsxParser(ctx, nil)
		assert.NoError(t, err)

		docs, err := p.Parse(ctx, f, parser.WithExtraMeta(map[string]any{"test": "test"}))
		assert.NoError(t, err)
		assert.True(t, len(docs) > 0)
		assert.True(t, len(docs[0].Content) > 0)
		assert.Equal(t, map[string]any{"县（区）": "新密市", "市": "郑州市", "省": "河南省"}, docs[0].MetaData)
	})
}
