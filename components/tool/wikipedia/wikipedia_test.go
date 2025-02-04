package wikipedia

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia/wikipediaclient"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewTool(t *testing.T) {
	ctx := context.Background()
	tool, err := NewTool(ctx, &Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestWikipedia_Search(t *testing.T) {
	ctx := context.Background()
	tool, err := NewTool(ctx, &Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
	test := []struct {
		name  string
		query *SearchRequest
		err   error
	}{
		{"normal1", &SearchRequest{"bytedance"}, nil},
		{"normal2", &SearchRequest{"Go programming language"}, nil},
		{"InvalidParameters", &SearchRequest{""}, fmt.Errorf("[LocalFunc] failed to invoke tool: %w", wikipediaclient.ErrInvalidParameters)},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			m, err := sonic.MarshalString(tt.query)
			assert.NoError(t, err)
			toolRes, err := tool.InvokableRun(ctx, m)
			assert.Equal(t, tt.err, err)
			assert.NotNil(t, toolRes)
		})
	}
}
