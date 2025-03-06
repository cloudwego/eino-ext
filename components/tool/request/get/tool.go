package get

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Config struct {
	ToolName            string            `json:"tool_name"`
	ToolDesc            string            `json:"tool_desc"`
	Headers             map[string]string `json:"headers"`
	Timeout             time.Duration     `json:"timeout"`
	ResponseContentType string            `json:"response_content_type"`
}

func (c *Config) validate() error {
	if c.ToolName == "" {
		c.ToolName = "request_get"
	}
	if c.ToolDesc == "" {
		c.ToolDesc = `A portal to the internet. Use this when you need to get specific
		content from a website. Input should be a URL (i.e. https://www.google.com).
		The output will be the text response of the GET request.`
	}
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.ResponseContentType == "" {
		c.ResponseContentType = "text"
	} else if c.ResponseContentType != "text" && c.ResponseContentType != "json" {
		return errors.New("invalid response_content_type, it must be 'text' or 'json'")
	}
	return nil
}

func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	reqTool, err := newRequestTool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create request tool: %w", err)
	}

	invokableTool, err := utils.InferTool(config.ToolName, config.ToolDesc, reqTool.Get)
	if err != nil {
		return nil, fmt.Errorf("failed to infer the tool: %w", err)
	}

	return invokableTool, nil
}

type GetRequestTool struct {
	config *Config
	client *http.Client
}

func newRequestTool(config *Config) (*GetRequestTool, error) {
	if config == nil {
		return nil, errors.New("request tool configuration is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: &http.Transport{},
	}

	return &GetRequestTool{
		config: config,
		client: client,
	}, nil
}
