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

package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volc-sdk-golang/base"
)

const (
	path           = "/api/knowledge/collection/search_knowledge"
	defaultBaseURL = "api-knowledgebase.mlp.cn-beijing.volces.com"
)

type Config struct {
	// Timeout specifies the duration to wait before timing out a request
	// Optional. Default: 0 (no timeout)
	Timeout time.Duration

	// BaseURL is the base URL for the knowledge API
	// Optional. Default: "api-knowledgebase.mlp.cn-beijing.volces.com"
	BaseURL string

	// AK specifies the access key for authentication
	// Required
	AK string

	// SK specifies the secret key for authentication
	// Required
	SK string

	// AccountID specifies the unique identifier for your Volcengine account
	// Required
	AccountID string

	// The following fields are from the Volcengine Knowledge search_knowledge openapi request body.
	// For more details, please refer to: https://www.volcengine.com/docs/84313/1350012

	// Name specifies the name of the knowledge collection
	// Optional.
	// Note: Either Name+Project pair or ResourceID must be provided, but not both
	Name string

	// Project specifies the project identifier
	// Optional. Default: "default"
	// Note: Either Name+Project pair or ResourceID must be provided, but not both
	Project string

	// ResourceID specifies the resource identifier
	// Optional.
	// Note: Either Name+Project pair or ResourceID must be provided, but not both
	ResourceID string

	// Limit specifies the maximum number of documents to retrieve
	// Optional. Range: [1, 200], Default: 10
	Limit int32

	// DocFilter specifies filters to apply to the document search
	// Optional. Default: nil
	DocFilter map[string]any

	// DenseWeight specifies the weight for dense retrieval
	// Optional. Default: 0.5
	DenseWeight float64

	// NeedInstruction specifies whether instructions are needed in the response
	// Optional.
	NeedInstruction bool

	// Rewrite specifies whether to rewrite the query
	// Optional.
	Rewrite bool

	// ReturnTokenUsage specifies whether to return token usage information
	// Optional.
	ReturnTokenUsage bool

	// Messages specifies the list of historical conversation messages
	// Only required when rewriting is enabled (Rewrite=true)
	// Used to rewrite queries based on conversation history
	// Optional.
	Messages []*schema.Message

	// RerankSwitch specifies whether to enable reranking of results
	// Optional.
	RerankSwitch bool

	// RetrieveCount specifies the number of documents to retrieve for reranking
	// Only takes effect when RerankSwitch is true
	// Must be greater than or equal to Limit, otherwise an error will be returned
	// Optional. Default: 25
	RetrieveCount int32

	// ChunkDiffusionCount specifies the number of chunks for diffusion
	// Optional.
	ChunkDiffusionCount int32

	// ChunkGroup specifies whether to group chunks in the response
	// Optional.
	ChunkGroup bool

	// RerankModel specifies the model to use for reranking results
	// Only takes effect when RerankSwitch is set to true
	// Optional.
	RerankModel string

	// RerankOnlyChunk specifies whether to calculate reranking scores based only on chunk content
	// True: Calculate scores using only chunk content
	// False: Calculate scores using both chunk title and content
	// Optional.
	RerankOnlyChunk bool

	// GetAttachmentLink specifies whether to include attachment links in the response
	// Optional.
	GetAttachmentLink bool
}

func NewRetriever(ctx context.Context, conf *Config) (retriever.Retriever, error) {
	nConf := *conf
	if len(nConf.BaseURL) == 0 {
		nConf.BaseURL = defaultBaseURL
	}
	cli := http.DefaultClient
	if nConf.Timeout > 0 {
		cli = &http.Client{Timeout: nConf.Timeout}
	}
	return &knowledge{
		cfg: &nConf,
		cli: cli,
		credential: &base.Credentials{
			AccessKeyID:     nConf.AK,
			SecretAccessKey: nConf.SK,
			Service:         "air",
			Region:          "cn-north-1",
		},
	}, nil
}

type knowledge struct {
	cfg        *Config
	cli        *http.Client
	credential *base.Credentials
}

func (k *knowledge) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	origReq := &request{
		Name:        k.cfg.Name,
		Project:     k.cfg.Project,
		ResourceID:  k.cfg.ResourceID,
		Query:       query,
		Limit:       k.cfg.Limit,
		DenseWeight: k.cfg.DenseWeight,
		PreProcessing: preProcessing{
			NeedInstruction:  k.cfg.NeedInstruction,
			Rewrite:          k.cfg.Rewrite,
			ReturnTokenUsage: k.cfg.ReturnTokenUsage,
			Messages:         k.cfg.Messages,
		},
		PostProcessing: postProcessing{
			RerankSwitch:        k.cfg.RerankSwitch,
			RetrieveCount:       k.cfg.RetrieveCount,
			ChunkDiffusionCount: k.cfg.ChunkDiffusionCount,
			ChunkGroup:          k.cfg.ChunkGroup,
			RerankModel:         k.cfg.RerankModel,
			RerankOnlyChunk:     k.cfg.RerankOnlyChunk,
			GetAttachmentLink:   k.cfg.GetAttachmentLink,
		},
	}
	if k.cfg.DocFilter != nil {
		origReq.QueryParam = &queryParam{DocFilter: k.cfg.DocFilter}
	}

	body, err := sonic.Marshal(origReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request fail: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, (&url.URL{
		Scheme: "https",
		Host:   k.cfg.BaseURL,
		Path:   path,
	}).String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request fail: %w", err)
	}

	resp, err := k.cli.Do(k.prepareRequest(req).WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("do request fail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve documents, code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response fail: %w", err)
	}
	origResp := &response{}
	if err = sonic.Unmarshal(respBody, origResp); err != nil {
		return nil, fmt.Errorf("unmarshal response fail: %w", err)
	}
	if origResp.Code != 0 {
		return nil, fmt.Errorf("request fail, code: %d, msg: %s, request id: %s", origResp.Code, origResp.Message, origResp.RequestID)
	}
	return origResp.toDocuments(), nil
}

func (k *knowledge) prepareRequest(req *http.Request) *http.Request {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", k.cfg.BaseURL)
	req.Header.Set("V-Account-Id", k.cfg.AccountID)
	return k.credential.Sign(req)
}

type request struct {
	Name           string         `json:"name,omitempty"`
	Project        string         `json:"project,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	Query          string         `json:"query"`
	Limit          int32          `json:"limit,omitempty"`
	QueryParam     *queryParam    `json:"query_param,omitempty"`
	DenseWeight    float64        `json:"dense_weight,omitempty"`
	PreProcessing  preProcessing  `json:"pre_processing,omitempty"`
	PostProcessing postProcessing `json:"post_processing,omitempty"`
}

type queryParam struct {
	DocFilter map[string]any `json:"doc_filter,omitempty"`
}

type preProcessing struct {
	NeedInstruction  bool              `json:"need_instruction,omitempty"`
	Rewrite          bool              `json:"rewrite,omitempty"`
	ReturnTokenUsage bool              `json:"return_token_usage,omitempty"`
	Messages         []*schema.Message `json:"messages,omitempty"`
}

type postProcessing struct {
	RerankSwitch        bool   `json:"rerank_switch,omitempty"`
	RetrieveCount       int32  `json:"retrieve_count,omitempty"`
	ChunkDiffusionCount int32  `json:"chunk_diffusion_count,omitempty"`
	ChunkGroup          bool   `json:"chunk_group,omitempty"`
	RerankModel         string `json:"rerank_model,omitempty"`
	RerankOnlyChunk     bool   `json:"rerank_only_chunk,omitempty"`
	GetAttachmentLink   bool   `json:"get_attachment_link,omitempty"`
}

func (r *response) toDocuments() []*schema.Document {
	if r.Data == nil {
		return nil
	}
	docs := make([]*schema.Document, 0, len(r.Data.ResultList))
	for _, res := range r.Data.ResultList {
		doc := &schema.Document{
			ID:       res.ID,
			Content:  res.Content,
			MetaData: make(map[string]any),
		}
		setDocID(doc, res.DocInfo.DocID)
		setDocName(doc, res.DocInfo.DocName)
		setChunkID(doc, res.ChunkID)
		setAttachments(doc, res.ChunkAttachment)
		setTableChunks(doc, res.TableChunkFields)

		docs = append(docs, doc)
	}
	return docs
}

type response struct {
	Code      int32  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      *data  `json:"data"`
}

type data struct {
	CollectionName string        `json:"collection_name"`
	Count          int32         `json:"count"`
	RewriteQuery   string        `json:"rewrite_query"`
	TokenUsage     []*tokenUsage `json:"token_usage"`
	ResultList     []*result     `json:"result_list"`
}

type tokenUsage struct {
	EmbeddingTokenUsage schema.TokenUsage `json:"embedding_token_usage"`
	RerankTokenUsage    int32             `json:"rerank_token_usage"`
	RewriteTokenUsage   int32             `json:"rewrite_token_usage"`
}

type result struct {
	ID               string             `json:"id"`
	Content          string             `json:"content"`
	Score            float64            `json:"score"`
	PointID          string             `json:"point_id"`
	ChunkTitle       string             `json:"chunk_title"`
	ChunkID          int32              `json:"chunk_id"`
	ProcessTime      int64              `json:"process_time"`
	RerankScore      float64            `json:"rerank_score"`
	DocInfo          docInfo            `json:"doc_info"`
	RecallPosition   int                `json:"recall_position"`
	TableChunkFields []*TableChunkField `json:"table_chunk_fields"`
	OriginalQuestion string             `json:"original_question"`
	ChunkType        string             `json:"chunk_type"`
	ChunkAttachment  []*ChunkAttachment `json:"chunk_attachment"`
}

type docInfo struct {
	DocID      string `json:"doc_id"`
	DocName    string `json:"doc_name"`
	CreateTime int64  `json:"create_time"`
	DocType    string `json:"doc_type"`
	DocMeta    string `json:"doc_meta"`
	Source     string `json:"source"`
	Title      string `json:"title"`
}

type TableChunkField struct {
	FieldName  string          `json:"field_name"`
	FieldValue json.RawMessage `json:"field_value"`
}

type ChunkAttachment struct {
	UUID    string `json:"uuid"`
	Caption string `json:"caption"`
	Type    string `json:"type"`
	Link    string `json:"link"` // type 为 image 时表示图片的临时下载链接，有效期 10 分钟
}
