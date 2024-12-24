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

// Package ddgsearch provides a Go client for DuckDuckGo search API.
//
// Example usage:
//
//	cfg := &ddgsearch.Config{
//		Headers: map[string]string{
//			"User-Agent": "MyApp/1.0",
//		},
//		Timeout: 10 * time.Second,
//		Cache:   true,
//		MaxRetries: 3,
//	}
//
//	client, err := ddgsearch.New(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	params := &ddgsearch.SearchParams{
//		Query:      "golang programming",
//		Region:     ddgsearch.RegionUS,
//		SafeSearch: ddgsearch.SafeSearchModerate,
//		TimeRange:  ddgsearch.TimeRangeYear,
//		MaxResults: 10,
//	}
//
//	results, err := client.Search(context.Background(), params)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for _, result := range results {
//		fmt.Printf("Title: %s\nURL: %s\nDescription: %s\n\n",
//			result.Title, result.URL, result.Description)
//	}
package ddgsearch
