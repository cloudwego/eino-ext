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

package googlesearch

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Config struct {
	APIKey         string `json:"api_key"`
	SearchEngineID string `json:"search_engine_id"`
	BaseURL        string `json:"base_url"` // default: https://customsearch.googleapis.com
	Num            int    `json:"num"`
	Lang           string `json:"lang"` // usually represented by a 2-4 letter code in ISO 639-1. e.g. en，ja, zh-CN

	ToolName string `json:"tool_name"` // default: google_search
	ToolDesc string `json:"tool_desc"` // default: "custom search json api of google search engine"
}

func NewTool(ctx context.Context, conf *Config) (tool.InvokableTool, error) {

	if conf.APIKey == "" || conf.SearchEngineID == "" {
		return nil, fmt.Errorf("missing api_key or search_engine_id")
	}

	toolName := "google_search"
	toolDesc := "custom search json api of google search engine"
	if conf.ToolName != "" {
		toolName = conf.ToolName
	}
	if conf.ToolDesc != "" {
		toolDesc = conf.ToolDesc
	}

	cliOpts := make([]option.ClientOption, 0, 5)
	cliOpts = append(cliOpts, option.WithAPIKey(conf.APIKey))
	if conf.BaseURL != "" {
		cliOpts = append(cliOpts, option.WithEndpoint(conf.BaseURL))
	}

	cseSvr, err := customsearch.NewService(ctx, cliOpts...)
	if err != nil {
		return nil, err
	}

	gs := &googleSearch{
		conf:   conf,
		cseSvr: cseSvr,
	}

	tl, err := utils.InferTool(toolName, toolDesc,
		gs.search, utils.WithMarshalOutput(gs.marshalOutput))
	if err != nil {
		return nil, err
	}

	return tl, nil
}

type googleSearch struct {
	conf   *Config
	cseSvr *customsearch.Service
}

func (gs *googleSearch) search(ctx context.Context, req *SearchRequest) (*customsearch.Search, error) {

	num := req.Num
	if num <= 0 {
		num = gs.conf.Num
	}
	offset := req.Offset

	lang := req.Lang
	if lang == "" {
		lang = gs.conf.Lang
	}

	cseCall := gs.cseSvr.Cse.List().Context(ctx).Cx(gs.conf.SearchEngineID).Q(req.Query)
	if num > 0 {
		cseCall = cseCall.Num(int64(num))
	}
	if lang != "" {
		cseCall = cseCall.Gl(lang)
	}
	if offset > 0 {
		cseCall = cseCall.Start(int64(offset))
	}

	sc, err := cseCall.Do()
	if err != nil {
		return nil, fmt.Errorf("search.cse.list failed: %w", err)
	}

	return sc, nil
}

func (gs *googleSearch) marshalOutput(_ context.Context, output any) (string, error) {
	gsr, ok := output.(*customsearch.Search)
	if !ok {
		return "", fmt.Errorf("unexpected google search response, expect %T but given %T", gsr, output)
	}

	simpleItems := make([]*SimplifiedSearchItem, 0, len(gsr.Items))
	for _, item := range gsr.Items {
		ssi := &SimplifiedSearchItem{
			Link:    item.Link,
			Title:   item.Title,
			Snippet: item.Snippet,
		}
		desc, okk, err := getDescFromPageMap(item.Pagemap)
		if err != nil {
			return "", err
		}
		if okk {
			ssi.Desc = desc
		}

		simpleItems = append(simpleItems, ssi)
	}

	sr := SearchResult{
		Query: getQuery(gsr.Queries.Request),
		Items: simpleItems,
	}

	return sonic.MarshalString(sr)
}

func getQuery(reqs []*customsearch.SearchQueriesRequest) string {
	var sb strings.Builder
	isFirst := true
	for _, r := range reqs {
		if !isFirst {
			sb.WriteString(" ")
		}
		sb.WriteString(r.SearchTerms)
		isFirst = false
	}

	return sb.String()
}

func getDescFromPageMap(pageMap googleapi.RawMessage) (string, bool, error) {

	var pages map[string]any
	err := sonic.Unmarshal([]byte(pageMap), &pages)
	if err != nil {
		return "", false, fmt.Errorf("json.Unmarshal Pagemap failed: %w", err)
	}
	const (
		metaTagsKey = "metatags"
		descTag     = "description"
		ogDescTag   = "og:description"
	)
	metaTags, ok := pages[metaTagsKey].([]interface{})
	if !ok {
		return "", false, nil
	}

	var siteDesc strings.Builder
	foundDesc := false

	for _, mt := range metaTags {
		metas, okk := mt.(map[string]any)
		if !okk {
			continue
		}

		if desc, okk := metas[descTag].(string); okk {
			if foundDesc {
				siteDesc.WriteString("\n")
			}
			siteDesc.WriteString(desc)

			foundDesc = true
			continue
		}

		if desc, okk := metas[ogDescTag].(string); okk {
			if foundDesc {
				siteDesc.WriteString("\n")
			}
			siteDesc.WriteString(desc)

			foundDesc = true
		}
	}

	return siteDesc.String(), foundDesc, nil
}

type SearchRequest struct {
	Query  string `json:"query" jsonschema_description:"queried string to the search engine"`
	Num    int    `json:"num,omitempty" jsonschema_description:"number of search results to return, valid values are between 1 and 10, inclusive"`
	Offset int    `json:"offset,omitempty" jsonschema_description:"the index of the first result to return."`
	Lang   string `json:"lang,omitempty" jsonschema_description:"sets the user interface language, default english. usually represented by a 2-4 letter code in ISO 639-1. e.g. en, ja, zh-CN"`
}

type SearchResult struct {
	Query string                  `json:"query,omitempty"`
	Items []*SimplifiedSearchItem `json:"items"`
}

type SimplifiedSearchItem struct {
	Link    string `json:"link"`
	Title   string `json:"title,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	Desc    string `json:"desc,omitempty"`
}
