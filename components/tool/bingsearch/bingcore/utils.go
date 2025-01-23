package bingcore

import (
	"fmt"
	"github.com/bytedance/sonic"
)

// BingAnswer represents the response from Bing search API.
func parseSearchResponse(body []byte) ([]*SearchResult, error) {
	var response BingAnswer
	err := sonic.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	results := make([]*SearchResult, 0, len(response.WebPages.Value))
	for _, resp := range response.WebPages.Value {
		results = append(results, &SearchResult{
			Title:       resp.Name,
			URL:         resp.URL,
			Description: resp.Snippet,
		})
	}
	return results, nil
}
