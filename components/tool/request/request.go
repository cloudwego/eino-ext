package request

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/request/get"
	"github.com/cloudwego/eino-ext/components/tool/request/post"
	"github.com/cloudwego/eino/components/tool"
)

type Config struct {
	Headers             map[string]string `json:"headers"`
	Timeout             time.Duration     `json:"timeout"`
	ResponseContentType string            `json:"response_content_type"`
}

func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	getConf := &get.Config{
		Headers:             conf.Headers,
		Timeout:             conf.Timeout,
		ResponseContentType: conf.ResponseContentType,
	}
	getTool, err := get.NewTool(ctx, getConf)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar ferramenta GET: %w", err)
	}

	postConf := &post.Config{
		Headers:             conf.Headers,
		Timeout:             conf.Timeout,
		ResponseContentType: conf.ResponseContentType,
	}
	postTool, err := post.NewTool(ctx, postConf)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar ferramenta POST: %w", err)
	}

	return []tool.BaseTool{getTool, postTool}, nil
}
