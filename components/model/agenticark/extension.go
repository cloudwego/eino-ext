/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticark

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/cloudwego/eino/schema"
	"github.com/go-viper/mapstructure/v2"
)

type ResponseMetaExtension struct {
	ID                 string             `json:"id,omitempty" mapstructure:"id,omitempty"`
	Status             ResponseStatus     `json:"status,omitempty" mapstructure:"status,omitempty"`
	IncompleteDetails  *IncompleteDetails `json:"incomplete_details" mapstructure:"incomplete_details,omitempty"`
	Error              *ResponseError     `json:"error" mapstructure:"error,omitempty"`
	PreviousResponseID string             `json:"previous_response_id,omitempty" mapstructure:"previous_response_id,omitempty"`
	Thinking           *ResponseThinking  `json:"thinking,omitempty" mapstructure:"thinking,omitempty"`
	ExpireAt           *int64             `json:"expire_at,omitempty" mapstructure:"expire_at,omitempty"`
	ServiceTier        ServiceTier        `json:"service_tier,omitempty" mapstructure:"service_tier,omitempty"`

	StreamingError *StreamingResponseError `json:"streaming_error,omitempty" mapstructure:"streaming_error,omitempty"`
}

type AssistantGenTextExtension struct {
	Annotations []*TextAnnotation `json:"annotations,omitempty" mapstructure:"annotations,omitempty"`
}

type ServerToolCallArguments struct {
	WebSearch       *WebSearchArguments       `json:"web_search,omitempty" mapstructure:"web_search,omitempty"`
	ImageProcess    *ImageProcessArguments    `json:"image_process,omitempty" mapstructure:"image_process,omitempty"`
	DoubaoApp       *DoubaoAppArguments       `json:"doubao_app,omitempty" mapstructure:"doubao_app,omitempty"`
	KnowledgeSearch *KnowledgeSearchArguments `json:"knowledge_search,omitempty" mapstructure:"knowledge_search,omitempty"`
}

type KnowledgeSearchArguments struct {
	KnowledgeResourceID string   `json:"knowledge_resource_id,omitempty" mapstructure:"knowledge_resource_id,omitempty"`
	Queries             []string `json:"queries,omitempty" mapstructure:"queries,omitempty"`
}

type ServerToolResult struct {
	ImageProcess *ImageProcessResult `json:"image_process,omitempty" mapstructure:"image_process,omitempty"`
	DoubaoApp    *DoubaoAppResult    `json:"doubao_app,omitempty" mapstructure:"doubao_app,omitempty"`
}

type DoubaoAppArguments struct {
	Feature DoubaoAppFeature `json:"feature,omitempty" mapstructure:"feature,omitempty"`
}

type DoubaoAppResult struct {
	Blocks []*DoubaoAppBlock `json:"blocks,omitempty" mapstructure:"blocks,omitempty"`
}

type DoubaoAppBlock struct {
	// StreamingMeta contains streaming metadata for this block.
	// Only present when processing streaming response.
	StreamingMeta *DoubaoAppStreamingMeta `json:"streaming_meta,omitempty" mapstructure:"streaming_meta,omitempty"`

	Type            DoubaoAppBlockType        `json:"type,omitempty" mapstructure:"type,omitempty"`
	OutputText      *DoubaoAppOutputText      `json:"output_text,omitempty" mapstructure:"output_text,omitempty"`
	ReasoningText   *DoubaoAppReasoningText   `json:"reasoning_text,omitempty" mapstructure:"reasoning_text,omitempty"`
	Search          *DoubaoAppSearch          `json:"search,omitempty" mapstructure:"search,omitempty"`
	ReasoningSearch *DoubaoAppReasoningSearch `json:"reasoning_search,omitempty" mapstructure:"reasoning_search,omitempty"`
}

// DoubaoAppStreamingMeta contains streaming metadata for DoubaoAppBlock.
type DoubaoAppStreamingMeta struct {
	// Index is the index of this block within DoubaoApp result.
	Index int64 `json:"index,omitempty" mapstructure:"index,omitempty"`
}

type DoubaoAppOutputText struct {
	ID       string `json:"id,omitempty" mapstructure:"id,omitempty"`
	ParentID string `json:"parent_id,omitempty" mapstructure:"parent_id,omitempty"`
	Text     string `json:"text,omitempty" mapstructure:"text,omitempty"`

	// Status represents the status of the output text.
	// It is only available in non-streaming response.
	Status string `json:"status,omitempty" mapstructure:"status,omitempty"`
}

type DoubaoAppReasoningText struct {
	ID            string `json:"id,omitempty" mapstructure:"id,omitempty"`
	ParentID      string `json:"parent_id,omitempty" mapstructure:"parent_id,omitempty"`
	ReasoningText string `json:"reasoning_text,omitempty" mapstructure:"reasoning_text,omitempty"`

	// Status represents the status of the reasoning text.
	// It is only available in non-streaming response.
	Status string `json:"status,omitempty" mapstructure:"status,omitempty"`
}

type DoubaoAppSearch struct {
	ID       string                   `json:"id,omitempty" mapstructure:"id,omitempty"`
	ParentID string                   `json:"parent_id,omitempty" mapstructure:"parent_id,omitempty"`
	Summary  string                   `json:"summary,omitempty" mapstructure:"summary,omitempty"`
	Queries  []string                 `json:"queries,omitempty" mapstructure:"queries,omitempty"`
	Results  []*DoubaoAppSearchResult `json:"results,omitempty" mapstructure:"results,omitempty"`

	// SearchingState represents the state of searching.
	// It is only available in streaming response.
	SearchingState string `json:"searching_state,omitempty" mapstructure:"searching_state,omitempty"`

	// Status represents the status of the search.
	// It is only available in non-streaming response.
	Status string `json:"status,omitempty" mapstructure:"status,omitempty"`
}

type DoubaoAppReasoningSearch struct {
	ID       string                   `json:"id,omitempty" mapstructure:"id,omitempty"`
	ParentID string                   `json:"parent_id,omitempty" mapstructure:"parent_id,omitempty"`
	Summary  string                   `json:"summary,omitempty" mapstructure:"summary,omitempty"`
	Queries  []string                 `json:"queries,omitempty" mapstructure:"queries,omitempty"`
	Results  []*DoubaoAppSearchResult `json:"results,omitempty" mapstructure:"results,omitempty"`

	// SearchingState represents the state of reasoning search.
	// It is only available in streaming response.
	SearchingState string `json:"searching_state,omitempty" mapstructure:"searching_state,omitempty"`

	// Status represents the status of the reasoning search.
	// It is only available in non-streaming response.
	Status string `json:"status,omitempty" mapstructure:"status,omitempty"`
}

type DoubaoAppSearchResult struct {
	Title    string `json:"title,omitempty" mapstructure:"title,omitempty"`
	URL      string `json:"url,omitempty" mapstructure:"url,omitempty"`
	SiteName string `json:"site_name,omitempty" mapstructure:"site_name,omitempty"`
}

func getResponseMeta(meta *schema.AgenticResponseMeta) *ResponseMetaExtension {
	if meta == nil || meta.Extension == nil {
		return nil
	}
	if ext, ok := meta.Extension.(*ResponseMetaExtension); ok {
		return ext
	}
	if m, ok := meta.Extension.(map[string]any); ok {
		ext := &ResponseMetaExtension{}
		if err := mapstructure.Decode(m, ext); err != nil {
			return nil
		}
		return ext
	}
	return nil
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments are nil")
	}
	if args, ok := call.Arguments.(*ServerToolCallArguments); ok {
		return args, nil
	}
	if m, ok := call.Arguments.(map[string]any); ok {
		args := &ServerToolCallArguments{}
		if err := mapstructure.Decode(m, args); err != nil {
			return nil, fmt.Errorf("failed to decode server tool call arguments: %w", err)
		}
		return args, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool call arguments", call.Arguments)
}

func getServerToolResult(res *schema.ServerToolResult) (*ServerToolResult, error) {
	if res == nil || res.Result == nil {
		return nil, fmt.Errorf("server tool result is nil")
	}
	if result, ok := res.Result.(*ServerToolResult); ok {
		return result, nil
	}
	if m, ok := res.Result.(map[string]any); ok {
		result := &ServerToolResult{}
		if err := mapstructure.Decode(m, result); err != nil {
			return nil, fmt.Errorf("failed to decode server tool result: %w", err)
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool result", res.Result)
}

type ResponseError struct {
	Code    string `json:"code,omitempty" mapstructure:"code,omitempty"`
	Message string `json:"message,omitempty" mapstructure:"message,omitempty"`
}

type StreamingResponseError struct {
	Code    string `json:"code,omitempty" mapstructure:"code,omitempty"`
	Message string `json:"message,omitempty" mapstructure:"message,omitempty"`
	Param   string `json:"param,omitempty" mapstructure:"param,omitempty"`
}

type IncompleteDetails struct {
	Reason        string         `json:"reason,omitempty" mapstructure:"reason,omitempty"`
	ContentFilter *ContentFilter `json:"content_filter,omitempty" mapstructure:"content_filter,omitempty"`
}

type ContentFilter struct {
	Type    string `json:"type,omitempty" mapstructure:"type,omitempty"`
	Details string `json:"details,omitempty" mapstructure:"details,omitempty"`
}

type ResponseThinking struct {
	Type ThinkingType `json:"type,omitempty" mapstructure:"type,omitempty"`
}

type WebSearchArguments struct {
	ActionType WebSearchAction `json:"action_type,omitempty" mapstructure:"action_type,omitempty"`
	Search     *WebSearchQuery `json:"search,omitempty" mapstructure:"search,omitempty"`
}

type WebSearchQuery struct {
	Query string `json:"query,omitempty" mapstructure:"query,omitempty"`
}

type ImageProcessArguments struct {
	ActionType ImageProcessAction     `json:"action_type,omitempty" mapstructure:"action_type,omitempty"`
	Point      *ImageProcessPoint     `json:"point,omitempty" mapstructure:"point,omitempty"`
	Grounding  *ImageProcessGrounding `json:"grounding,omitempty" mapstructure:"grounding,omitempty"`
	Rotate     *ImageProcessRotate    `json:"rotate,omitempty" mapstructure:"rotate,omitempty"`
	Zoom       *ImageProcessZoom      `json:"zoom,omitempty" mapstructure:"zoom,omitempty"`
}

type ImageProcessPoint struct {
	ImageIndex int32  `json:"image_index,omitempty" mapstructure:"image_index,omitempty"`
	Points     string `json:"points,omitempty" mapstructure:"points,omitempty"`
	DrawLine   bool   `json:"draw_line,omitempty" mapstructure:"draw_line,omitempty"`
}

type ImageProcessGrounding struct {
	ImageIndex int32  `json:"image_index,omitempty" mapstructure:"image_index,omitempty"`
	BboxStr    string `json:"bbox_str,omitempty" mapstructure:"bbox_str,omitempty"`
	Crop       bool   `json:"crop,omitempty" mapstructure:"crop,omitempty"`
}

type ImageProcessRotate struct {
	ImageIndex int32 `json:"image_index,omitempty" mapstructure:"image_index,omitempty"`
	Degree     int32 `json:"degree,omitempty" mapstructure:"degree,omitempty"`
}

type ImageProcessZoom struct {
	ImageIndex int32   `json:"image_index,omitempty" mapstructure:"image_index,omitempty"`
	BboxStr    string  `json:"bbox_str,omitempty" mapstructure:"bbox_str,omitempty"`
	Scale      float64 `json:"scale,omitempty" mapstructure:"scale,omitempty"`
}

type ImageProcessResult struct {
	Action *ImageProcessResultAction `json:"action,omitempty" mapstructure:"action,omitempty"`
	Error  *ImageProcessResultError  `json:"error,omitempty" mapstructure:"error,omitempty"`
}

type ImageProcessResultAction struct {
	Type           ImageProcessAction `json:"type,omitempty" mapstructure:"type,omitempty"`
	ResultImageURL string             `json:"result_image_url,omitempty" mapstructure:"result_image_url,omitempty"`
}

type ImageProcessResultError struct {
	Message string `json:"message,omitempty" mapstructure:"message,omitempty"`
}

type TextAnnotation struct {
	Index int `json:"index,omitempty" mapstructure:"index,omitempty"`

	Type TextAnnotationType `json:"type,omitempty" mapstructure:"type,omitempty"`

	URLCitation *URLCitation `json:"url_citation,omitempty" mapstructure:"url_citation,omitempty"`
	DocCitation *DocCitation `json:"doc_citation,omitempty" mapstructure:"doc_citation,omitempty"`
}

type URLCitation struct {
	Title         string      `json:"title,omitempty" mapstructure:"title,omitempty"`
	URL           string      `json:"url,omitempty" mapstructure:"url,omitempty"`
	LogoURL       string      `json:"logo_url,omitempty" mapstructure:"logo_url,omitempty"`
	MobileURL     string      `json:"mobile_url,omitempty" mapstructure:"mobile_url,omitempty"`
	SiteName      string      `json:"site_name,omitempty" mapstructure:"site_name,omitempty"`
	PublishTime   string      `json:"publish_time,omitempty" mapstructure:"publish_time,omitempty"`
	CoverImage    *CoverImage `json:"cover_image,omitempty" mapstructure:"cover_image,omitempty"`
	Summary       string      `json:"summary,omitempty" mapstructure:"summary,omitempty"`
	FreshnessInfo string      `json:"freshness_info,omitempty" mapstructure:"freshness_info,omitempty"`
}

type CoverImage struct {
	URL    string `json:"url,omitempty" mapstructure:"url,omitempty"`
	Width  *int64 `json:"width,omitempty" mapstructure:"width,omitempty"`
	Height *int64 `json:"height,omitempty" mapstructure:"height,omitempty"`
}

type DocCitation struct {
	DocID           string           `json:"doc_id,omitempty" mapstructure:"doc_id,omitempty"`
	DocName         string           `json:"doc_name,omitempty" mapstructure:"doc_name,omitempty"`
	ChunkID         *int32           `json:"chunk_id,omitempty" mapstructure:"chunk_id,omitempty"`
	ChunkAttachment []map[string]any `json:"chunk_attachment,omitempty" mapstructure:"chunk_attachment,omitempty"`
}

func concatResponseMetaExtensions(chunks []*ResponseMetaExtension) (ret *ResponseMetaExtension, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	ret = &ResponseMetaExtension{}

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.ID != "" {
			ret.ID = chunk.ID
		}
		if chunk.Status != "" {
			ret.Status = chunk.Status
		}
		if chunk.IncompleteDetails != nil {
			ret.IncompleteDetails = chunk.IncompleteDetails
		}
		if chunk.Error != nil {
			ret.Error = chunk.Error
		}
		if chunk.PreviousResponseID != "" {
			ret.PreviousResponseID = chunk.PreviousResponseID
		}
		if chunk.Thinking != nil {
			ret.Thinking = chunk.Thinking
		}
		if chunk.ExpireAt != nil {
			ret.ExpireAt = chunk.ExpireAt
		}
		if chunk.ServiceTier != "" {
			ret.ServiceTier = chunk.ServiceTier
		}
		if chunk.StreamingError != nil {
			ret.StreamingError = chunk.StreamingError
		}
	}

	return ret, nil
}

func concatAssistantGenTextExtensions(chunks []*AssistantGenTextExtension) (ret *AssistantGenTextExtension, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	ret = &AssistantGenTextExtension{}

	var allAnnotations []*TextAnnotation
	for _, ext := range chunks {
		allAnnotations = append(allAnnotations, ext.Annotations...)
	}

	var (
		indices           []int
		indexToAnnotation = map[int]*TextAnnotation{}
	)

	for _, an := range allAnnotations {
		if an == nil {
			continue
		}
		if indexToAnnotation[an.Index] == nil {
			indexToAnnotation[an.Index] = an
			indices = append(indices, an.Index)
		} else {
			return nil, fmt.Errorf("duplicate annotation index %d", an.Index)
		}
	}

	sort.Slice(indices, func(i, j int) bool {
		return indices[i] < indices[j]
	})

	ret.Annotations = make([]*TextAnnotation, 0, len(indices))
	for _, idx := range indices {
		an := *indexToAnnotation[idx]
		an.Index = 0 // clear index
		ret.Annotations = append(ret.Annotations, &an)
	}

	return ret, nil
}

func concatServerToolCallArguments(chunks []*ServerToolCallArguments) (ret *ServerToolCallArguments, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	var (
		expectedType          reflect.Type
		webSearchArguments    *WebSearchArguments
		imageProcessArguments *ImageProcessArguments
		doubaoAppArguments    []*DoubaoAppArguments
		knowledgeSearchArgs   *KnowledgeSearchArguments
	)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		switch {
		case chunk.WebSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.WebSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if webSearchArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple web search arguments")
			}
			webSearchArguments = chunk.WebSearch

		case chunk.ImageProcess != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.ImageProcess))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if imageProcessArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple image process arguments")
			}
			imageProcessArguments = chunk.ImageProcess

		case chunk.DoubaoApp != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.DoubaoApp))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			doubaoAppArguments = append(doubaoAppArguments, chunk.DoubaoApp)

		case chunk.KnowledgeSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.KnowledgeSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if knowledgeSearchArgs != nil {
				return nil, fmt.Errorf("cannot concat multiple knowledge search arguments")
			}
			knowledgeSearchArgs = chunk.KnowledgeSearch
		}
	}

	if webSearchArguments != nil {
		return &ServerToolCallArguments{WebSearch: webSearchArguments}, nil
	}
	if imageProcessArguments != nil {
		return &ServerToolCallArguments{ImageProcess: imageProcessArguments}, nil
	}
	if len(doubaoAppArguments) > 0 {
		return &ServerToolCallArguments{DoubaoApp: concatDoubaoAppArguments(doubaoAppArguments)}, nil
	}
	if knowledgeSearchArgs != nil {
		return &ServerToolCallArguments{KnowledgeSearch: knowledgeSearchArgs}, nil
	}

	return nil, fmt.Errorf("no valid server tool call arguments to concat")
}

func concatDoubaoAppArguments(chunks []*DoubaoAppArguments) *DoubaoAppArguments {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return chunks[0]
	}
	ret := &DoubaoAppArguments{}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.Feature != "" {
			ret.Feature = chunk.Feature
		}
	}
	return ret
}

func concatServerToolResult(chunks []*ServerToolResult) (ret *ServerToolResult, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	var (
		expectedType       reflect.Type
		imageProcessResult *ImageProcessResult
		doubaoAppResults   []*DoubaoAppResult
	)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		switch {
		case chunk.ImageProcess != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.ImageProcess))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			if imageProcessResult != nil {
				return nil, fmt.Errorf("cannot concat multiple image process results")
			}
			imageProcessResult = chunk.ImageProcess

		case chunk.DoubaoApp != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.DoubaoApp))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			doubaoAppResults = append(doubaoAppResults, chunk.DoubaoApp)
		}
	}

	if imageProcessResult != nil {
		return &ServerToolResult{ImageProcess: imageProcessResult}, nil
	}

	if len(doubaoAppResults) > 0 {
		da, err := concatDoubaoAppResults(doubaoAppResults)
		if err != nil {
			return nil, err
		}
		return &ServerToolResult{DoubaoApp: da}, nil
	}

	return nil, fmt.Errorf("no valid server tool result to concat")
}

func checkExpectedType(expectedType, chunkType reflect.Type) (reflect.Type, error) {
	if expectedType == nil {
		return chunkType, nil
	}
	if expectedType != chunkType {
		return nil, fmt.Errorf("type mismatch, expected '%s', but got '%s'", expectedType, chunkType)
	}
	return expectedType, nil
}

func concatDoubaoAppResults(chunks []*DoubaoAppResult) (*DoubaoAppResult, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	ret := &DoubaoAppResult{}
	var (
		blocks        []*DoubaoAppBlock
		blockIndices  []int64
		indexToBlocks = make(map[int64][]*DoubaoAppBlock)
	)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		for _, block := range chunk.Blocks {
			if block == nil {
				continue
			}
			if block.StreamingMeta == nil {
				if len(blockIndices) > 0 {
					return nil, fmt.Errorf("found non-streaming block after streaming blocks")
				}
				blocks = append(blocks, block)
			} else {
				if len(blocks) > 0 {
					return nil, fmt.Errorf("found streaming block after non-streaming blocks")
				}
				idx := block.StreamingMeta.Index
				if _, ok := indexToBlocks[idx]; !ok {
					blockIndices = append(blockIndices, idx)
				}
				indexToBlocks[idx] = append(indexToBlocks[idx], block)
			}
		}
	}

	if len(blocks) > 0 {
		ret.Blocks = blocks
		return ret, nil
	}

	if len(blockIndices) > 0 {
		indexToBlock := make(map[int64]*DoubaoAppBlock)
		for idx, bs := range indexToBlocks {
			indexToBlock[idx] = concatDoubaoAppBlocks(bs)
		}
		blocks = make([]*DoubaoAppBlock, 0, len(blockIndices))
		sort.Slice(blockIndices, func(i, j int) bool {
			return blockIndices[i] < blockIndices[j]
		})
		for _, idx := range blockIndices {
			blocks = append(blocks, indexToBlock[idx])
		}
		ret.Blocks = blocks
	}

	return ret, nil
}

func concatDoubaoAppBlocks(blocks []*DoubaoAppBlock) *DoubaoAppBlock {
	if len(blocks) == 0 {
		return nil
	}
	if len(blocks) == 1 {
		return blocks[0]
	}
	ret := &DoubaoAppBlock{}
	for _, block := range blocks {
		concatDoubaoAppBlock(ret, block)
	}
	return ret
}

func concatDoubaoAppBlock(dst, src *DoubaoAppBlock) {
	if src.Type != "" {
		dst.Type = src.Type
	}
	if src.OutputText != nil {
		dst.OutputText = concatDoubaoAppOutputText(dst.OutputText, src.OutputText)
	}
	if src.ReasoningText != nil {
		dst.ReasoningText = concatDoubaoAppReasoningText(dst.ReasoningText, src.ReasoningText)
	}
	if src.Search != nil {
		dst.Search = concatDoubaoAppSearch(dst.Search, src.Search)
	}
	if src.ReasoningSearch != nil {
		dst.ReasoningSearch = concatDoubaoAppReasoningSearch(dst.ReasoningSearch, src.ReasoningSearch)
	}
}

func concatDoubaoAppOutputText(dst, src *DoubaoAppOutputText) *DoubaoAppOutputText {
	if dst == nil {
		dst = &DoubaoAppOutputText{}
	}
	if src.ID != "" {
		dst.ID = src.ID
	}
	dst.Text += src.Text
	return dst
}

func concatDoubaoAppReasoningText(dst, src *DoubaoAppReasoningText) *DoubaoAppReasoningText {
	if dst == nil {
		dst = &DoubaoAppReasoningText{}
	}
	if src.ID != "" {
		dst.ID = src.ID
	}
	dst.ReasoningText += src.ReasoningText
	return dst
}

func concatDoubaoAppSearch(dst, src *DoubaoAppSearch) *DoubaoAppSearch {
	if dst == nil {
		dst = &DoubaoAppSearch{}
	}
	if src.ID != "" {
		dst.ID = src.ID
	}
	dst.Summary += src.Summary
	dst.Queries = append(dst.Queries, src.Queries...)
	dst.Results = append(dst.Results, src.Results...)
	return dst
}

func concatDoubaoAppReasoningSearch(dst, src *DoubaoAppReasoningSearch) *DoubaoAppReasoningSearch {
	if dst == nil {
		dst = &DoubaoAppReasoningSearch{}
	}
	if src.ID != "" {
		dst.ID = src.ID
	}
	dst.Summary += src.Summary
	dst.Queries = append(dst.Queries, src.Queries...)
	dst.Results = append(dst.Results, src.Results...)
	return dst
}
