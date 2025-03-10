package post

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
	// Inspired by the "Requests" tool from the LangChain project, specifically the RequestsPostTool.
	// For more details, visit: https://python.langchain.com/docs/integrations/tools/requests/
	// Optional. Default: "requests_post".
	ToolName string `json:"tool_name"`

	//  Optional. Default: Use this when you want to POST to a website.
	// 	Input should be a JSON string with two keys: "url" and "body".
	// 	The value of "url" should be a string, and the value of "body" should be a dictionary of
	// 	key-value pairs you want to POST to the URL.
	// 	Be careful to always use double quotes for strings in the JSON string.
	// 	The output will be the text response of the POST request.
	ToolDesc string `json:"tool_desc"`

	// Optional.
	// Headers is a map of HTTP header names to their corresponding values.
	// These headers will be included in every request made by the tool.
	Headers map[string]string `json:"headers"`

	// Optional.
	// HttpClient is the HTTP client used to perform the requests.
	// If not provided, a default client with a 30-second timeout and a standard transport
	// will be initialized and used.
	HttpClient *http.Client
}

func (c *Config) validate() error {
	if c.ToolName == "" {
		c.ToolName = "requests_post"
	}
	if c.ToolDesc == "" {
		c.ToolDesc = `Use this when you want to POST to a website.
		Input should be a JSON string with two keys: "url" and "body".
		The value of "url" should be a string, and the value of "body" should be a dictionary of 
		key-value pairs you want to POST to the URL.
		Be careful to always use double quotes for strings in the JSON string.
		The output will be the text response of the POST request.`
	}
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if c.HttpClient == nil {
		c.HttpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		}
	}
	return nil
}

func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	reqTool, err := newRequestTool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create request tool: %w", err)
	}

	invokableTool, err := utils.InferTool(config.ToolName, config.ToolDesc, reqTool.Post)
	if err != nil {
		return nil, fmt.Errorf("failed to infer the tool: %w", err)
	}

	return invokableTool, nil
}

type PostRequestTool struct {
	config *Config
	client *http.Client
}

func newRequestTool(config *Config) (*PostRequestTool, error) {
	if config == nil {
		return nil, errors.New("request tool configuration is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &PostRequestTool{
		config: config,
		client: config.HttpClient,
	}, nil
}
