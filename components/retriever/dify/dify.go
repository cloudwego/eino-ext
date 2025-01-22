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

package dify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"strings"
)

// Request Body
type Request struct {
	Query          string          `json:"query"`
	RetrievalModel *RetrievalModel `json:"retrieval_model,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type Query struct {
	Content string `json:"content"`
}

type Segment struct {
	Id            string    `json:"id"`
	Position      int       `json:"position"`
	DocumentId    string    `json:"document_id"`
	Content       string    `json:"content"`
	WordCount     int       `json:"word_count"`
	Tokens        int       `json:"tokens"`
	Keywords      []string  `json:"keywords"`
	IndexNodeId   string    `json:"index_node_id"`
	IndexNodeHash string    `json:"index_node_hash"`
	HitCount      int       `json:"hit_count"`
	Enabled       bool      `json:"enabled"`
	Status        string    `json:"status"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     int       `json:"created_at"`
	IndexingAt    int       `json:"indexing_at"`
	CompletedAt   int       `json:"completed_at"`
	Document      *Document `json:"document"`
}

type Document struct {
	Id             string `json:"id"`
	DataSourceType string `json:"data_source_type"`
	Name           string `json:"name"`
}

type Record struct {
	Segment *Segment
	Score   float64 `json:"score"`
}

type SuccessResponse struct {
	Query   *Query    `json:"query"`
	Records []*Record `json:"records"`
}

func (r *Retriever) getUrl() string {
	return strings.TrimRight(r.config.Endpoint, "/") + "/v1/datasets/" + r.config.DatasetID + "/retrieve"
}

func (r *Retriever) getAuth() string {
	return fmt.Sprintf("Bearer %s", r.config.APIKey)
}

func (r *Retriever) doPost(ctx context.Context, query string) (res *SuccessResponse, err error) {
	data := &Request{
		Query:          query,
		RetrievalModel: r.retrievalModel,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %w", err)
	}
	// 发送检索请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.getUrl(), bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Authorization", r.getAuth())
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()
	// 请求失败
	if resp.StatusCode != http.StatusOK {
		errResp := &ErrorResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("request failed: %s", errResp.Message)
		}
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	res = &SuccessResponse{}

	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return res, nil
}

func (x *Record) ToDoc() *schema.Document {
	if x == nil || x.Segment == nil {
		return nil
	}
	doc := &schema.Document{
		ID:       x.Segment.Id,
		Content:  x.Segment.Content,
		MetaData: map[string]any{},
	}
	doc.WithScore(x.Score)
	return doc
}
