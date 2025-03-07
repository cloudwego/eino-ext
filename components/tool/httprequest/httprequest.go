package httprequest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloudwego/eino-ext/components/tool/httprequest/delete"
	"github.com/cloudwego/eino-ext/components/tool/httprequest/get"
	"github.com/cloudwego/eino-ext/components/tool/httprequest/post"
	"github.com/cloudwego/eino-ext/components/tool/httprequest/put"

	"github.com/cloudwego/eino/components/tool"
)

type Config struct {
	Headers    map[string]string `json:"headers"`
	HttpClient *http.Client
}

func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	getConf := &get.Config{
		Headers:    conf.Headers,
		HttpClient: conf.HttpClient,
	}
	getTool, err := get.NewTool(ctx, getConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool GET: %w", err)
	}

	postConf := &post.Config{
		Headers:    conf.Headers,
		HttpClient: conf.HttpClient,
	}
	postTool, err := post.NewTool(ctx, postConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool POST: %w", err)
	}

	putConf := &put.Config{
		Headers:    conf.Headers,
		HttpClient: conf.HttpClient,
	}
	putTool, err := put.NewTool(ctx, putConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool PUT: %w", err)
	}

	deleteConf := &delete.Config{
		Headers:    conf.Headers,
		HttpClient: conf.HttpClient,
	}
	deleteTool, err := delete.NewTool(ctx, deleteConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool DELETE: %w", err)
	}

	return []tool.BaseTool{getTool, postTool, putTool, deleteTool}, nil
}
