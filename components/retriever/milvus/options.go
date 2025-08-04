/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package milvus

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type ImplOptions struct {
	limit        int
	hybridSearch []*HybridSearch
}

type HybridSearch struct {
	annField        string
	metricsType     entity.MetricType
	searchParam     map[string]string
	groupByField    string
	groupSize       int
	strictGroupSize bool
	annParam        index.AnnParam
	ignoreGrowing   bool
	expr            string
	topK            int
	offset          int
	templateParams  map[string]any
}

func WithLimit(limit int) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(opt *ImplOptions) {
		opt.limit = limit
	})
}

func WithHybridSearchOption(option ...*HybridSearch) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.hybridSearch = option
	})
}

func NewHybridSearchOption(annField string, limit int) *HybridSearch {
	return &HybridSearch{
		annField:       annField,
		topK:           limit,
		searchParam:    make(map[string]string),
		templateParams: make(map[string]any),
	}
}

func (h *HybridSearch) WithANNSField(annsField string) *HybridSearch {
	h.annField = annsField
	return h
}

func (h *HybridSearch) WithGroupByField(groupByField string) *HybridSearch {
	h.groupByField = groupByField
	return h
}

func (h *HybridSearch) WithGroupSize(groupSize int) *HybridSearch {
	h.groupSize = groupSize
	return h
}

func (h *HybridSearch) WithStrictGroupSize(strictGroupSize bool) *HybridSearch {
	h.strictGroupSize = strictGroupSize
	return h
}

func (h *HybridSearch) WithSearchParam(key, value string) *HybridSearch {
	h.searchParam[key] = value
	return h
}

func (h *HybridSearch) WithAnnParam(ap index.AnnParam) *HybridSearch {
	h.annParam = ap
	return h
}

func (h *HybridSearch) WithFilter(expr string) *HybridSearch {
	h.expr = expr
	return h
}

func (h *HybridSearch) WithTemplateParam(key string, val any) *HybridSearch {
	h.templateParams[key] = val
	return h
}

func (h *HybridSearch) WithOffset(offset int) *HybridSearch {
	h.offset = offset
	return h
}

func (h *HybridSearch) WithIgnoreGrowing(ignoreGrowing bool) *HybridSearch {
	h.ignoreGrowing = ignoreGrowing
	return h
}

func (h *HybridSearch) getAnnRequest(limit int, vectors []entity.Vector) *milvusclient.AnnRequest {
	if h.topK <= 0 {
		h.topK = limit
	}
	req := milvusclient.NewAnnRequest(h.annField, h.topK, vectors...).
		WithFilter(h.expr).
		WithAnnParam(h.annParam).
		WithIgnoreGrowing(h.ignoreGrowing).
		WithOffset(h.offset).
		WithGroupByField(h.groupByField).
		WithGroupSize(h.groupSize).
		WithStrictGroupSize(h.strictGroupSize)
	for k, v := range h.searchParam {
		req.WithSearchParam(k, v)
	}
	for k, v := range h.templateParams {
		req.WithTemplateParam(k, v)
	}
	return req
}
