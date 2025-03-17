/*
 * Copyright 2025 CloudWeGo Authors
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

func NewToolKit(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	getConf := &get.Config{}
	if conf != nil {
		getConf.Headers = conf.Headers
		getConf.HttpClient = conf.HttpClient
	}

	getTool, err := get.NewTool(ctx, getConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool GET: %w", err)
	}

	postConf := &post.Config{}
	if conf != nil {
		postConf.Headers = conf.Headers
		postConf.HttpClient = conf.HttpClient
	}
	postTool, err := post.NewTool(ctx, postConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool POST: %w", err)
	}

	putConf := &put.Config{}
	if conf != nil {
		putConf.Headers = conf.Headers
		putConf.HttpClient = conf.HttpClient
	}
	putTool, err := put.NewTool(ctx, putConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool PUT: %w", err)
	}

	deleteConf := &delete.Config{}
	if conf != nil {
		deleteConf.Headers = conf.Headers
		deleteConf.HttpClient = conf.HttpClient
	}
	deleteTool, err := delete.NewTool(ctx, deleteConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool DELETE: %w", err)
	}

	return []tool.BaseTool{getTool, postTool, putTool, deleteTool}, nil
}
