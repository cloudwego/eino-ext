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

package ddgsearch

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// extractVQD extracts the VQD token from the DuckDuckGo response
func extractVQD(body []byte, query string) (string, error) {
	content := string(body)

	// Try to find vqd in JavaScript code
	re := regexp.MustCompile(`vqd=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Try to find vqd in meta tags
	re = regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+name=["']vqd["']`)
	matches = re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Try to find vqd in any context
	re = regexp.MustCompile(`vqd=([^&"']+)`)
	matches = re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not extract vqd for keywords: %s", query)
}

// extractVQDToken extracts the VQD token from the HTML response
func extractVQDToken(html string) string {
	re := regexp.MustCompile(`vqd=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// normalizeURL removes AMP and tracking parameters from URLs.
func normalizeURL(u string) string {
	if u == "" {
		return ""
	}

	// Remove AMP
	u = strings.ReplaceAll(u, "/amp/", "/")
	u = strings.ReplaceAll(u, "?amp=1", "")
	u = strings.ReplaceAll(u, "&amp=1", "")

	// Parse URL
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}

	// Remove tracking parameters
	q := parsed.Query()
	for k := range q {
		if strings.Contains(strings.ToLower(k), "utm_") {
			q.Del(k)
		}
	}
	parsed.RawQuery = q.Encode()

	return parsed.String()
}

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}
