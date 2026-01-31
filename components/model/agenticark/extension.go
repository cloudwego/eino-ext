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
	"sort"

	"github.com/cloudwego/eino/schema"
)

type ResponseMetaExtension struct {
	ID                 string             `json:"id,omitempty"`
	Status             ResponseStatus     `json:"status,omitempty"`
	IncompleteDetails  *IncompleteDetails `json:"incomplete_details"`
	Error              *ResponseError     `json:"error"`
	PreviousResponseID string             `json:"previous_response_id,omitempty"`
	Thinking           *ResponseThinking  `json:"thinking,omitempty"`
	ExpireAt           *int64             `json:"expire_at,omitempty"`
	ServiceTier        ServiceTier        `json:"service_tier,omitempty"`

	StreamingError *StreamingResponseError `json:"streaming_error,omitempty"`
}

type AssistantGenTextExtension struct {
	Annotations []*TextAnnotation `json:"annotations,omitempty"`
}

type ServerToolCallArguments struct {
	WebSearch *WebSearchArguments `json:"web_search,omitempty"`
}

func getResponseMeta(meta *schema.AgenticResponseMeta) *ResponseMetaExtension {
	if meta == nil || meta.Extension == nil {
		return nil
	}
	return meta.Extension.(*ResponseMetaExtension)
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments is nil")
	}
	arguments, ok := call.Arguments.(*ServerToolCallArguments)
	if !ok {
		return nil, fmt.Errorf("expected '*ServerToolCallArguments', but got '%T'", call.Arguments)
	}
	return arguments, nil
}

type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type StreamingResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   string `json:"param,omitempty"`
}

type IncompleteDetails struct {
	Reason        string         `json:"reason,omitempty"`
	ContentFilter *ContentFilter `json:"content_filter,omitempty"`
}

type ContentFilter struct {
	Type    string `json:"type,omitempty"`
	Details string `json:"details,omitempty"`
}

type ResponseThinking struct {
	Type ThinkingType `json:"type,omitempty"`
}

type WebSearchArguments struct {
	ActionType WebSearchAction `json:"action_type,omitempty"`

	Search *WebSearchQuery `json:"search,omitempty"`
}

type WebSearchQuery struct {
	Query string `json:"query,omitempty"`
}

type TextAnnotation struct {
	Index int `json:"index,omitempty"`

	Type TextAnnotationType `json:"type,omitempty"`

	URLCitation *URLCitation `json:"url_citation,omitempty"`
	DocCitation *DocCitation `json:"doc_citation,omitempty"`
}

type URLCitation struct {
	Title         string      `json:"title,omitempty"`
	URL           string      `json:"url,omitempty"`
	LogoURL       string      `json:"logo_url,omitempty"`
	MobileURL     string      `json:"mobile_url,omitempty"`
	SiteName      string      `json:"site_name,omitempty"`
	PublishTime   string      `json:"publish_time,omitempty"`
	CoverImage    *CoverImage `json:"cover_image,omitempty"`
	Summary       string      `json:"summary,omitempty"`
	FreshnessInfo string      `json:"freshness_info,omitempty"`
}

type CoverImage struct {
	URL    string `json:"url,omitempty"`
	Width  *int64 `json:"width,omitempty"`
	Height *int64 `json:"height,omitempty"`
}

type DocCitation struct {
	DocID           string           `json:"doc_id,omitempty"`
	DocName         string           `json:"doc_name,omitempty"`
	ChunkID         *int32           `json:"chunk_id,omitempty"`
	ChunkAttachment []map[string]any `json:"chunk_attachment,omitempty"`
}

func concatResponseMetaExtensions(chunks []*ResponseMetaExtension) (ret *ResponseMetaExtension, err error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no response meta extension found")
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
		return nil, fmt.Errorf("no assistant generated text extension found")
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
		return nil, fmt.Errorf("no server tool call arguments found")
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}
	return nil, fmt.Errorf("cannot concat multiple server tool call arguments")
}
