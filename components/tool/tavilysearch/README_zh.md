# Tavily 搜索工具

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Tavily 搜索工具。该工具实现了 `InvokableTool` 接口，可以与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/tavilysearch
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/tavilysearch"
)

func main() {
	ctx := context.Background()
	country := "china"
	ts, err := tavilysearch.NewTool(ctx, &tavilysearch.Config{
		APIKey:  os.Getenv("TAVILY_API_KEY"),
		Country: &country,
	})
	if err != nil {
		log.Fatalf("create tavily search tool failed, %v", err)
		return
	}
	req := tavilysearch.SearchRequest{
		Query: "What is transformer?",
	}
	reqStr, _ := sonic.MarshalString(&req)
	tout, err := ts.InvokableRun(ctx, reqStr)
	if err != nil {
		log.Fatalf("invokable run failed, %v", err)
		return
	}
	fmt.Println(tout)
}

```

## 配置

该工具可以使用 `Config` 结构体进行配置：

```go
type Config struct {
	// Eino tool settings
	ToolName string `json:"tool_name"` // optional, default is "tavily_search"
	ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by tavily"

	// Tavily search settings
	// APIKey The API key is required to access the Tavily Search API.
	APIKey string `json:"api_key"`

	// When auto_parameters is enabled, Tavily automatically configures search parameters based on your query's content and intent.
	// You can still set other parameters manually, and your explicit values will override the automatic ones.
	// The parameters include_answer, include_raw_content, and max_results must always be set manually, as they directly affect response size.
	// Note: search_depth may be automatically set to advanced when it's likely to improve results. This uses 2 API credits per request.
	// To avoid the extra cost, you can explicitly set search_depth to basic. Currently in BETA.
	// default:false
	AutoParameters *bool `json:"auto_parameters,omitempty"`

	// The category of the search.news is useful for retrieving real-time updates, particularly about politics, sports,
	// and major current events covered by mainstream media sources. general is for broader, more general-purpose searches that may
	// include a wide range of sources.
	// Available options: general, news, default:general
	Topic *string `json:"topic,omitempty"`

	// The depth of the search. advanced search is tailored to retrieve the most relevant sources and content snippets for your query,
	// while basic search provides generic content snippets from each source. A basic search costs 1 API Credit,
	// while an advanced search costs 2 API Credits.
	// Available options: basic, advanced, default:basic
	SearchDepth *string `json:"search_depth,omitempty"`

	// Chunks are short content snippets (maximum 500 characters each) pulled directly from the source. Use chunks_per_source to define
	// the maximum number of relevant chunks returned per source and to control the content length. Chunks will appear in the content
	// field as: <chunk 1> [...] <chunk 2> [...] <chunk 3>. Available only when search_depth is advanced.
	// Required range: 1 <= x <= 3, default:3
	ChunksPerSource *int `json:"chunks_per_source,omitempty"`

	// The maximum number of search results to return.
	// Required range: 1 <= x <= 20, default:5
	MaxResults *int `json:"max_results,omitempty"`

	// The time range back from the current date to filter results. Useful when looking for sources that have published data.
	// Available options: day, week, month, year, d, w, m, y
	TimeRange *string `json:"time_range,omitempty"`

	// Number of days back from the current date to include. Available only if topic is news.
	// Required range: x >= 1, default:7
	Days *int `json:"days,omitempty"`

	// Include an LLM-generated answer to the provided query. basic or true returns a quick answer. advanced returns a more detailed answer.
	// default:false
	IncludeAnswer *bool `json:"include_answer,omitempty"`

	// Include the cleaned and parsed HTML content of each search result. markdown or true returns search result content in markdown format.
	// text returns the plain text from the results and may increase latency.
	// default:false
	IncludeRawContent *bool `json:"include_raw_content,omitempty"`

	// Also perform an image search and include the results in the response.
	// default:false
	IncludeImages *bool `json:"include_images,omitempty"`

	// When include_images is true, also add a descriptive text for each image.
	// default:false
	IncludeImageDescriptions *bool `json:"include_image_descriptions,omitempty"`

	// A list of domains to specifically include in the search results.
	IncludeDomains []string `json:"include_domains,omitempty"`

	// A list of domains to specifically exclude from the search results.
	ExcludeDomains []string `json:"exclude_domains,omitempty"`

	// Boost search results from a specific country. This will prioritize content from the selected country in the search results.
	// Available only if topic is general.
	// Available options: afghanistan, albania, algeria, andorra, angola, argentina, armenia, australia, austria, azerbaijan, bahamas, bahrain,
	// bangladesh, barbados, belarus, belgium, belize, benin, bhutan, bolivia, bosnia and herzegovina, botswana, brazil, brunei, bulgaria,
	// burkina faso, burundi, cambodia, cameroon, canada, cape verde, central african republic, chad, chile, china, colombia, comoros, congo,
	// costa rica, croatia, cuba, cyprus, czech republic, denmark, djibouti, dominican republic, ecuador, egypt, el salvador, equatorial guinea,
	// eritrea, estonia, ethiopia, fiji, finland, france, gabon, gambia, georgia, germany, ghana, greece, guatemala, guinea, haiti, honduras,
	// hungary, iceland, india, indonesia, iran, iraq, ireland, israel, italy, jamaica, japan, jordan, kazakhstan, kenya, kuwait, kyrgyzstan,
	// latvia, lebanon, lesotho, liberia, libya, liechtenstein, lithuania, luxembourg, madagascar, malawi, malaysia, maldives, mali, malta,
	// mauritania, mauritius, mexico, moldova, monaco, mongolia, montenegro, morocco, mozambique, myanmar, namibia, nepal, netherlands, new zealand,
	// nicaragua, niger, nigeria, north korea, north macedonia, norway, oman, pakistan, panama, papua new guinea, paraguay, peru, philippines,
	// poland, portugal, qatar, romania, russia, rwanda, saudi arabia, senegal, serbia, singapore, slovakia, slovenia, somalia, south africa,
	// south korea, south sudan, spain, sri lanka, sudan, sweden, switzerland, syria, taiwan, tajikistan, tanzania, thailand, togo, trinidad and tobago,
	// tunisia, turkey, turkmenistan, uganda, ukraine, united arab emirates, united kingdom, united states, uruguay, uzbekistan, venezuela, vietnam,
	// yemen, zambia, zimbabwe
	Country *string `json:"country,omitempty"`

	// HTTP client settings
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Optional, default: map[string]string{}
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; rv:11.0) like Gecko",
	//     "Accept-Language": "en-US",
	//   }
	Headers map[string]string `json:"headers"`

	// Timeout specifies the maximum duration for a single request.
	// Optional, default: 0(never timeout)
	// Example: 5 * time.Second
	Timeout time.Duration `json:"timeout"`
}
```

## Search

### 请求 Schema

```go
type SearchRequest struct {
	Query string `json:"query" jsonschema:"description=The search query to execute with Tavily."`
}
```

### 响应 Schema

```go
type SearchResult struct {
	Title      string  `json:"title" jsonschema:"description=The title of the search result."`
	URL        string  `json:"url" jsonschema:"description=The URL of the search result."`
	Content    string  `json:"content" jsonschema:"description=A short description of the search result."`
	Score      float64 `json:"score" jsonschema:"description=The relevance score of the search result."`
	RawContent string  `json:"raw_content" jsonschema:"description=The cleaned and parsed HTML content of the search result. Only if include_raw_content is true."`
}

type ImageResult struct {
	URL         string `json:"url" jsonschema:"description=Image url"`
	Description string `json:"description" jsonschema:"description=Image description"`
}

type SearchResponse struct {
	Query   string          `json:"query" jsonschema:"The search query that was executed."`
	Answer  string          `json:"answer" jsonschema:"A short answer to the user's query, generated by an LLM. Included in the response only if include_answer is requested (i.e., set to true, basic, or advanced)"`
	Results []*SearchResult `json:"results" jsonschema:"description=A list of sorted search results, ranked by relevancy."`
	Images  []*ImageResult  `json:"images" jsonschema:"description=List of query-related images. If include_image_descriptions is true, each item will have url and description."`
}
```

## 更多详情

- [Tavily Search API Documentation](https://docs.tavily.com/documentation/api-reference/endpoint/search)
- [Eino Documentation](https://github.com/cloudwego/eino)
