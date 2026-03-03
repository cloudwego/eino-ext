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
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const (
	origDocIDKey   = "orig_doc_id"
	origDocNameKey = "orig_doc_name"
	keywordsKey    = "keywords"
)

type RetrievalModel struct {
	SearchMethod          SearchMethod    `json:"search_method"`
	RerankingEnable       *bool           `json:"reranking_enable"`
	RerankingMode         *string         `json:"reranking_mode"`
	RerankingModel        *RerankingModel `json:"reranking_model"`
	Weights               *float64        `json:"weights"`
	TopK                  *int            `json:"top_k"`
	ScoreThresholdEnabled *bool           `json:"score_threshold_enabled"`
	ScoreThreshold        *float64        `json:"score_threshold"`
}

type RerankingModel struct {
	RerankingProviderName string `json:"reranking_provider_name"`
	RerankingModelName    string `json:"reranking_model_name"`
}

func (x *RerankingModel) copy() *RerankingModel {
	if x == nil {
		return nil
	}
	return &RerankingModel{
		RerankingProviderName: x.RerankingProviderName,
		RerankingModelName:    x.RerankingModelName,
	}
}
func copyPtr[T any](ptr *T) *T {
	if ptr == nil {
		return nil
	}
	val := *ptr
	return &val
}

func (x *RetrievalModel) copy() *RetrievalModel {
	if x == nil {
		return nil
	}
	return &RetrievalModel{
		SearchMethod:          x.SearchMethod,
		RerankingEnable:       copyPtr(x.RerankingEnable),
		RerankingMode:         copyPtr(x.RerankingMode),
		RerankingModel:        x.RerankingModel.copy(),
		Weights:               copyPtr(x.Weights),
		TopK:                  copyPtr(x.TopK),
		ScoreThresholdEnabled: copyPtr(x.ScoreThresholdEnabled),
		ScoreThreshold:        copyPtr(x.ScoreThreshold),
	}
}

// request Body
type request struct {
	Query          string          `json:"query"`
	RetrievalModel *RetrievalModel `json:"retrieval_model,omitempty"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type Query struct {
	Content string `json:"content"`
}

type Segment struct {
	ID            string    `json:"id"`
	Position      int       `json:"position"`
	DocumentID    string    `json:"document_id"`
	Content       string    `json:"content"`
	SignContent   string    `json:"sign_content"`
	WordCount     int       `json:"word_count"`
	Tokens        int       `json:"tokens"`
	Keywords      []string  `json:"keywords"`
	IndexNodeID   string    `json:"index_node_id"`
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
	ID             string `json:"id"`
	DataSourceType string `json:"data_source_type"`
	Name           string `json:"name"`
}

type Record struct {
	Segment *Segment
	Score   float64 `json:"score"`
}

type successResponse struct {
	Query   *Query    `json:"query"`
	Records []*Record `json:"records"`
}

func (r *Retriever) getURL() string {
	return strings.TrimRight(r.config.Endpoint, "/") + "/datasets/" + r.config.DatasetID + "/retrieve"
}

func (r *Retriever) getAuth() string {
	return fmt.Sprintf("Bearer %s", r.config.APIKey)
}

func (r *Retriever) getRequest(query string, option *retriever.Options) *request {
	// 避免污染原始数据，这里必须copy一次
	rm := r.config.RetrievalModel.copy()
	if rm != nil {
		// options 配置优先
		rm.TopK = option.TopK
		rm.ScoreThreshold = option.ScoreThreshold
	}
	return &request{
		Query:          query,
		RetrievalModel: rm,
	}
}

func (r *Retriever) doPost(ctx context.Context, query string, option *retriever.Options) (res *successResponse, err error) {
	reqData, err := sonic.MarshalString(r.getRequest(query, option))
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %w", err)
	}
	// 发送检索请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.getURL(), strings.NewReader(reqData))
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
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	// 请求失败
	if resp.StatusCode != http.StatusOK {
		errResp := &errorResponse{}
		if err = sonic.Unmarshal(body, errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("request failed: %s", errResp.Message)
		}
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}
	res = &successResponse{}
	if err = sonic.Unmarshal(body, res); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return res, nil
}

func (x *Record) toDoc() *schema.Document {
	if x == nil || x.Segment == nil {
		return nil
	}

	content := x.Segment.Content
	if x.Segment.SignContent != "" {
		content = x.Segment.SignContent
	}
	doc := &schema.Document{
		ID:       x.Segment.ID,
		Content:  content,
		MetaData: map[string]any{},
	}
	doc.WithScore(x.Score)
	setOrgDocID(doc, x.Segment.DocumentID)
	setKeywords(doc, x.Segment.Keywords)
	if x.Segment.Document != nil {
		setOrgDocName(doc, x.Segment.Document.Name)
	}
	return doc
}

func setOrgDocID(doc *schema.Document, id string) {
	if doc == nil {
		return
	}
	doc.MetaData[origDocIDKey] = id
}

func setOrgDocName(doc *schema.Document, name string) {
	if doc == nil {
		return
	}
	doc.MetaData[origDocNameKey] = name
}

func setKeywords(doc *schema.Document, keywords []string) {
	if doc == nil {
		return
	}
	doc.MetaData[keywordsKey] = keywords
}

func GetOrgDocID(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if v, ok := doc.MetaData[origDocIDKey]; ok {
		return v.(string)
	}
	return ""
}

func GetOrgDocName(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if v, ok := doc.MetaData[origDocNameKey]; ok {
		return v.(string)
	}
	return ""
}

func GetKeywords(doc *schema.Document) []string {
	if doc == nil {
		return nil
	}
	if v, ok := doc.MetaData[keywordsKey]; ok {
		return v.([]string)
	}
	return nil
}
