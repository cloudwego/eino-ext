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

package duckduckgo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/corpix/uarand"
)

// reference
// - https://github.com/deedy5/duckduckgo_search/blob/main/duckduckgo_search/duckduckgo_search.py
// - https://github.com/searxng/searxng/blob/master/searx/engines/duckduckgo.py

func (c *client) TextSearch(ctx context.Context, input *TextSearchRequest) (*TextSearchResponse, error) {
	if input.Query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	results := make([]*TextSearchResult, 0, c.maxResults)

	header := buildTextHTMLRequestHeader()
	reqBody := input.buildTextHTMLRequestBody(c.region)

	for {
		var req *http.Request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchHTMLURL, strings.NewReader(reqBody.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header = header

		resultsTmp, nextReqBody, err := c.doTextHTMLSearch(ctx, req)
		if err != nil {
			return nil, err
		}

		if len(resultsTmp) == 0 {
			break
		}

		results = append(results, resultsTmp...)
		reqBody = nextReqBody

		if len(results) >= c.maxResults {
			results = results[:c.maxResults]
			break
		}

		if len(reqBody) == 0 {
			break
		}

		<-time.After(3 * time.Second) // request too fast may cause 202
	}

	if len(results) == 0 {
		return &TextSearchResponse{
			Message: "No good results were found.",
		}, nil
	}

	resp := &TextSearchResponse{
		Message: fmt.Sprintf("Found %d results successfully.", len(results)),
		Results: results,
	}

	return resp, nil
}

func buildTextHTMLRequestHeader() http.Header {
	return http.Header{
		"Referer":        {"https://html.duckduckgo.com/"},
		"Sec-Fetch-Site": {"same-origin"},
		"Sec-Fetch-Dest": {"document"},
		"Sec-Fetch-Mode": {"navigate"},
		"Sec-Fetch-User": {"?1"},
		"Content-Type":   {"application/x-www-form-urlencoded"},
		"User-Agent":     {uarand.GetRandom()},
	}
}

func (t *TextSearchRequest) buildTextHTMLRequestBody(region Region) url.Values {
	// q (str): Search query string
	// s (int): Search offset for pagination
	// nextParams (str): Continuation parameters from previous page response, typically empty
	// v (str): Typically 'l' for subsequent pages
	// o (str): Output format, typically 'json'
	// dc (int): Display count - value equal to offset (s) + 1
	// api (str): API endpoint identifier, typically 'd.js'
	// vqd (str): Validation query digest
	// kl (str): Keyboard language/region code (e.g., 'en-us')
	// df (str): Time filter, maps to values like 'd' (day), 'w' (week), 'm' (month), 'y' (year)

	body := url.Values{
		"q":  {t.Query},
		"b":  {""},
		"kl": {""},
		"df": {string(TimeRangeAny)},
	}

	if region != RegionWT {
		body["kl"] = []string{string(region)}
	}

	switch t.TimeRange {
	case TimeRangeDay, TimeRangeWeek, TimeRangeMonth, TimeRangeYear:
		body["df"] = []string{string(t.TimeRange)}
	}

	return body
}

func (c *client) doTextHTMLSearch(ctx context.Context, req *http.Request) (results []*TextSearchResult, nextReqBody url.Values, err error) {
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	results, nextReqBody, err = parseTextHTMLSearchResponse(string(respBody))
	if err != nil {
		return nil, nil, err
	}

	return
}

func parseTextHTMLSearchResponse(respBody string) (results []*TextSearchResult, nextReqBody url.Values, err error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(respBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var elements []*goquery.Selection
	doc.Find("table").Last().Find("tr").Each(func(i int, s *goquery.Selection) {
		elements = append(elements, s)
	})

	hrefCache := make(map[string]bool)
	results = make([]*TextSearchResult, 0, len(elements))

	doc.Find("div#links div.web-result").Each(func(i int, s *goquery.Selection) {
		title := s.Find("h2.result__title > a").First()
		if title.Length() == 0 {
			return
		}

		href, _ := title.Attr("href")
		if href == "" {
			return
		}

		if _, ok := hrefCache[href]; ok ||
			strings.HasPrefix(href, "http://www.google.com/search?q=") ||
			strings.HasPrefix(href, "https://duckduckgo.com/y.js?ad_domain") {
			return
		}

		summary := s.Find("a.result__snippet").First()
		if summary.Length() == 0 {
			return
		}

		hrefCache[href] = true

		results = append(results, &TextSearchResult{
			Title:   strings.TrimSpace(title.Text()),
			URL:     href,
			Summary: strings.TrimSpace(summary.Text()),
		})
	})

	navLinks := doc.Find("div.nav-link")
	if navLinks.Length() == 0 {
		return results, nil, nil
	}

	nextReqBody = url.Values{}

	lastForm := doc.Find("form").Last()
	lastForm.Find("input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		name, nameExist := s.Attr("name")
		value, valueExist := s.Attr("value")
		if nameExist && valueExist {
			nextReqBody.Set(name, value)
		}
	})

	return results, nextReqBody, nil
}
