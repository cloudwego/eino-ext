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

package tencentsearch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	wsa "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/wsa/v20250508"
)

const (
	defaultEndpoint = "wsa.tencentcloudapi.com"
	defaultCnt      = uint64(10)
	defaultMode     = int64(0)
)

var validCntValues = map[uint64]struct{}{
	10: {},
	20: {},
	30: {},
	40: {},
	50: {},
}

var validModeValues = map[int64]struct{}{
	0: {},
	1: {},
	2: {},
}

type Config struct {
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
	Endpoint  string `json:"endpoint"` // default: "wsa.tencentcloudapi.com"
	Mode      int64  `json:"mode"`     // default: 0
	Cnt       uint64 `json:"cnt"`      // default: 10

	ToolName string `json:"tool_name"`
	ToolDesc string `json:"tool_desc"`
}

type SearchRequest struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"queried string to the search engine"`
	Mode  *int64 `json:"mode,omitempty" jsonschema_description:"0-natural, 1-VR, 2-mixed"`

	// Site limits natural search results to the given domain.
	Site *string `json:"site,omitempty" jsonschema_description:"site domain to search within"`
	// FromTime filters natural search results by start timestamp in seconds.
	FromTime *int64 `json:"from_time,omitempty" jsonschema_description:"start time timestamp in seconds"`
	// ToTime filters natural search results by end timestamp in seconds.
	ToTime *int64 `json:"to_time,omitempty" jsonschema_description:"end time timestamp in seconds"`
	// Industry filters by premium-only categories: gov/news/acad/finance.
	Industry *string `json:"industry,omitempty" jsonschema_description:"industry filter, one of gov/news/acad/finance, premium only"`
	Cnt      *uint64 `json:"cnt,omitempty" jsonschema_description:"number of search results to return, 10/20/30/40/50"`
}

type SimplifiedSearchItem struct {
	Title   string   `json:"title,omitempty"`
	Date    string   `json:"date,omitempty"`
	URL     string   `json:"url,omitempty"`
	Passage string   `json:"passage,omitempty"`
	Content string   `json:"content,omitempty"`
	Site    string   `json:"site,omitempty"`
	Score   float64  `json:"score,omitempty"`
	Images  []string `json:"images,omitempty"`
	Favicon string   `json:"favicon,omitempty"`
}

type SearchResult struct {
	Query     string                  `json:"query,omitempty"`
	Items     []*SimplifiedSearchItem `json:"items"`
	Version   string                  `json:"version,omitempty"`
	Msg       string                  `json:"msg,omitempty"`
	RequestId string                  `json:"request_id,omitempty"`
}

type tencentSearch struct {
	conf   *Config
	client *wsa.Client
}

func validateCnt(cnt uint64) error {
	if _, ok := validCntValues[cnt]; ok {
		return nil
	}
	return fmt.Errorf("invalid cnt: %d, valid values are 10/20/30/40/50", cnt)
}

func validateMode(mode int64) error {
	if _, ok := validModeValues[mode]; ok {
		return nil
	}
	return fmt.Errorf("invalid mode: %d, valid values are 0/1/2", mode)
}

func NewTool(ctx context.Context, conf *Config) (tool.InvokableTool, error) {
	if conf == nil {
		conf = &Config{}
	}
	if conf.SecretID == "" {
		return nil, fmt.Errorf("secret_id is required")
	}
	if conf.SecretKey == "" {
		return nil, fmt.Errorf("secret_key is required")
	}
	if conf.Cnt == 0 {
		conf.Cnt = defaultCnt
	} else if err := validateCnt(conf.Cnt); err != nil {
		return nil, err
	}
	if conf.Mode != defaultMode {
		if err := validateMode(conf.Mode); err != nil {
			return nil, err
		}
	}

	toolName := "tencent_search"
	toolDesc := "Search using Tencent Cloud Web Search API"
	if conf.ToolName != "" {
		toolName = conf.ToolName
	}
	if conf.ToolDesc != "" {
		toolDesc = conf.ToolDesc
	}

	endpoint := defaultEndpoint
	if conf.Endpoint != "" {
		endpoint = conf.Endpoint
	}

	credential := common.NewCredential(conf.SecretID, conf.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = endpoint

	// Set Scheme to HTTP if testing with local mock server
	if endpoint != defaultEndpoint {
		cpf.HttpProfile.Scheme = "HTTP"
	}

	client, err := wsa.NewClient(credential, "", cpf)
	if err != nil {
		return nil, fmt.Errorf("create wsa client failed: %w", err)
	}

	ss := &tencentSearch{
		conf:   conf,
		client: client,
	}

	return utils.InferTool(toolName, toolDesc, ss.search)
}

func (s *tencentSearch) search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	wsaReq := wsa.NewSearchProRequest()
	wsaReq.Query = common.StringPtr(req.Query)

	if req.Mode != nil {
		if err := validateMode(*req.Mode); err != nil {
			return nil, err
		}
		wsaReq.Mode = req.Mode
	} else if s.conf.Mode != defaultMode {
		wsaReq.Mode = common.Int64Ptr(s.conf.Mode)
	} else {
		wsaReq.Mode = common.Int64Ptr(defaultMode)
	}

	if req.Cnt != nil {
		if err := validateCnt(*req.Cnt); err != nil {
			return nil, err
		}
		wsaReq.Cnt = common.Uint64Ptr(*req.Cnt)
	} else {
		wsaReq.Cnt = common.Uint64Ptr(s.conf.Cnt)
	}

	if req.Site != nil {
		wsaReq.Site = req.Site
	}
	if req.FromTime != nil {
		wsaReq.FromTime = req.FromTime
	}
	if req.ToTime != nil {
		wsaReq.ToTime = req.ToTime
	}
	if req.Industry != nil {
		wsaReq.Industry = req.Industry
	}

	wsaResp, err := s.client.SearchProWithContext(ctx, wsaReq)
	if err != nil {
		return nil, fmt.Errorf("wsa SearchPro failed: %w", err)
	}

	if wsaResp == nil || wsaResp.Response == nil {
		return nil, fmt.Errorf("empty response from wsa")
	}

	res := &SearchResult{
		Items: make([]*SimplifiedSearchItem, 0, len(wsaResp.Response.Pages)),
	}

	if wsaResp.Response.Query != nil {
		res.Query = *wsaResp.Response.Query
	} else {
		res.Query = req.Query
	}
	if wsaResp.Response.Version != nil {
		res.Version = *wsaResp.Response.Version
	}
	if wsaResp.Response.Msg != nil {
		res.Msg = *wsaResp.Response.Msg
	}
	if wsaResp.Response.RequestId != nil {
		res.RequestId = *wsaResp.Response.RequestId
	}

	for _, pageStr := range wsaResp.Response.Pages {
		if pageStr == nil {
			continue
		}
		var item SimplifiedSearchItem
		if err := json.Unmarshal([]byte(*pageStr), &item); err != nil {
			// skip invalid json
			continue
		}
		res.Items = append(res.Items, &item)
	}

	return res, nil
}
