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

package internal

import "time"

// SearchResult represents a search result from the Wikipedia.
type SearchResult struct {
	Title     string `json:"title"`
	PageID    int    `json:"pageid"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	WordCount int    `json:"wordcount"`
	Language  string `json:"language"`
}

// Page represents a Wikipedia page.
type Page struct {
	Title       string    `json:"title"`
	PageID      int       `json:"pageid"`
	Content     string    `json:"content"`
	URL         string    `json:"url"`
	LastUpdated time.Time `json:"last_updated"`
}
