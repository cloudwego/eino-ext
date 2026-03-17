package sougousearch

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
	Query    string  `json:"query" jsonschema_description:"queried string to the search engine"`
	Mode     *int64  `json:"mode,omitempty" jsonschema_description:"0-natural, 1-VR, 2-mixed"`
	Site     *string `json:"site,omitempty" jsonschema_description:"site domain to search within"`
	FromTime *int64  `json:"from_time,omitempty" jsonschema_description:"start time timestamp in seconds"`
	ToTime   *int64  `json:"to_time,omitempty" jsonschema_description:"end time timestamp in seconds"`
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

type sougouSearch struct {
	conf   *Config
	client *wsa.Client
}

func NewTool(ctx context.Context, conf *Config) (tool.InvokableTool, error) {
	if conf == nil {
		conf = &Config{}
	}

	toolName := "sougou_search"
	toolDesc := "Search using Sougou search engine via Tencent Cloud API"
	if conf.ToolName != "" {
		toolName = conf.ToolName
	}
	if conf.ToolDesc != "" {
		toolDesc = conf.ToolDesc
	}

	endpoint := "wsa.tencentcloudapi.com"
	if conf.Endpoint != "" {
		endpoint = conf.Endpoint
	}

	credential := common.NewCredential(conf.SecretID, conf.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = endpoint

	// Set Scheme to HTTP if testing with local mock server
	if endpoint != "wsa.tencentcloudapi.com" {
		cpf.HttpProfile.Scheme = "HTTP"
	}

	client, err := wsa.NewClient(credential, "", cpf)
	if err != nil {
		return nil, fmt.Errorf("create wsa client failed: %w", err)
	}

	ss := &sougouSearch{
		conf:   conf,
		client: client,
	}

	return utils.InferTool(toolName, toolDesc, ss.search)
}

func (s *sougouSearch) search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	wsaReq := wsa.NewSearchProRequest()
	wsaReq.Query = common.StringPtr(req.Query)

	if req.Mode != nil {
		wsaReq.Mode = req.Mode
	} else if s.conf.Mode > 0 {
		wsaReq.Mode = common.Int64Ptr(s.conf.Mode)
	} else {
		wsaReq.Mode = common.Int64Ptr(0)
	}

	if req.Cnt != nil {
		wsaReq.Cnt = req.Cnt
	} else if s.conf.Cnt > 0 {
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
