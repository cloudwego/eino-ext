package request

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	get "github.com/cloudwego/eino-ext/components/tool/request/get"
	post "github.com/cloudwego/eino-ext/components/tool/request/post"
)

func TestGetNewTool(t *testing.T) {
	ctx := context.Background()
	tool, err := get.NewTool(ctx, &get.Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestPostNewTool(t *testing.T) {
	ctx := context.Background()
	tool, err := post.NewTool(ctx, &post.Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
}
