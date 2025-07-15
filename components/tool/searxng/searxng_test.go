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

package searxng

import (
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func Test_NewClient(t *testing.T) {
	mockey.PatchConvey("Test NewClient", t, func() {

		mockey.PatchConvey("nil config", func() {
			client, err := NewClient(nil)
			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, "config is nil", err.Error())
		})

		mockey.PatchConvey("default config", func() {
			config := &ClientConfig{
				BaseUrl: "https://searx.example.com/search",
			}

			client, err := NewClient(config)
			assert.NoError(t, err)
			assert.NotNil(t, client)

			assert.Equal(t, "https://searx.example.com/search", client.config.BaseUrl)
			assert.Equal(t, 30*time.Second, client.config.Timeout)
			assert.Equal(t, 3, client.config.MaxRetries)
			assert.NotNil(t, client.config.Headers)
			assert.NotNil(t, client.client)
		})

		mockey.PatchConvey("custom config", func() {
			customConfig := &ClientConfig{
				BaseUrl:    "https://custom.searx.com/search",
				Timeout:    15 * time.Second,
				MaxRetries: 5,
				Headers: map[string]string{
					"User-Agent": "Custom-Agent",
				},
			}

			client, err := NewClient(customConfig)
			assert.NoError(t, err)
			assert.NotNil(t, client)

			assert.Equal(t, "https://custom.searx.com/search", client.config.BaseUrl)
			assert.Equal(t, 15*time.Second, client.config.Timeout)
			assert.Equal(t, 5, client.config.MaxRetries)
			assert.Equal(t, "Custom-Agent", client.config.Headers["User-Agent"])
			assert.Equal(t, 15*time.Second, client.client.Timeout)
		})
	})
}

func Test_SearchRequest_validate(t *testing.T) {
	mockey.PatchConvey("Test SearchRequest validate", t, func() {
		mockey.PatchConvey("empty query", func() {
			req := &SearchRequest{
				Query:  "",
				PageNo: 1,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "query is required", err.Error())
		})

		mockey.PatchConvey("invalid pageno", func() {
			req := &SearchRequest{
				Query:  "test",
				PageNo: 0,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "pageno must be greater than 0", err.Error())
		})

		mockey.PatchConvey("invalid time_range", func() {
			invalidTimeRange := "invalid"
			req := &SearchRequest{
				Query:     "test",
				PageNo:    1,
				TimeRange: &invalidTimeRange,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "time_range must be one of: [day month year]", err.Error())
		})

		mockey.PatchConvey("invalid language", func() {
			invalidLanguage := "invalid"
			req := &SearchRequest{
				Query:    "test",
				PageNo:   1,
				Language: &invalidLanguage,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "language must be one of: [all en zh zh-CN zh-TW fr de es ja ko ru ar pt it nl pl tr]", err.Error())
		})

		mockey.PatchConvey("invalid safesearch", func() {
			invalidSafeSearch := 3
			req := &SearchRequest{
				Query:      "test",
				PageNo:     1,
				SafeSearch: &invalidSafeSearch,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "safesearch must be one of: [0 1 2]", err.Error())
		})

		mockey.PatchConvey("invalid engines", func() {
			invalidEngines := "invalid"
			req := &SearchRequest{
				Query:   "test",
				PageNo:  1,
				Engines: &invalidEngines,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "engine 'invalid' is not supported. Valid engines are: [google duckduckgo baidu bing 360search yahoo quark]", err.Error())
		})

		mockey.PatchConvey("valid multiple engines", func() {
			multipleEngines := "google,duckduckgo,baidu"
			req := &SearchRequest{
				Query:   "test",
				PageNo:  1,
				Engines: &multipleEngines,
			}
			err := req.validate()
			assert.NoError(t, err)
		})

		mockey.PatchConvey("invalid multiple engines", func() {
			mixedEngines := "google,invalid,baidu"
			req := &SearchRequest{
				Query:   "test",
				PageNo:  1,
				Engines: &mixedEngines,
			}
			err := req.validate()
			assert.Error(t, err)
			assert.Equal(t, "engine 'invalid' is not supported. Valid engines are: [google duckduckgo baidu bing 360search yahoo quark]", err.Error())
		})

		mockey.PatchConvey("valid request", func() {
			timeRange := "day"
			language := "en"
			safeSearch := 1
			engines := "google"
			req := &SearchRequest{
				Query:      "test query",
				PageNo:     1,
				TimeRange:  &timeRange,
				Language:   &language,
				SafeSearch: &safeSearch,
				Engines:    &engines,
			}
			err := req.validate()
			assert.NoError(t, err)
		})
	})
}

func Test_BuildSearchInvokeTool(t *testing.T) {
	mockey.PatchConvey("Test BuildSearchInvokeTool", t, func() {
		mockey.PatchConvey("nil config", func() {
			tool, err := BuildSearchInvokeTool(nil)
			assert.Error(t, err)
			assert.Nil(t, tool)
		})

		mockey.PatchConvey("valid config", func() {
			config := &ClientConfig{
				BaseUrl: "https://searx.example.com/search",
			}

			tool, err := BuildSearchInvokeTool(config)
			assert.NoError(t, err)
			assert.NotNil(t, tool)
		})
	})
}

func Test_BuildSearchStreamTool(t *testing.T) {
	mockey.PatchConvey("Test BuildSearchStreamTool", t, func() {
		mockey.PatchConvey("nil config", func() {
			tool, err := BuildSearchStreamTool(nil)
			assert.Error(t, err)
			assert.Nil(t, tool)
		})

		mockey.PatchConvey("valid config", func() {
			config := &ClientConfig{
				BaseUrl: "https://searx.example.com/search",
			}

			tool, err := BuildSearchStreamTool(config)
			assert.NoError(t, err)
			assert.NotNil(t, tool)
		})
	})
}

func Test_getSearchSchema(t *testing.T) {
	mockey.PatchConvey("Test getSearchSchema", t, func() {
		schema := getSearchSchema()
		assert.NotNil(t, schema)
		assert.Equal(t, "web_search", schema.Name)
		assert.Contains(t, schema.Desc, "Performs a web search using the SearXNG API")
		assert.NotNil(t, schema.ParamsOneOf)
	})
}