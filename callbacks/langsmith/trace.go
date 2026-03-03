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

package langsmith

import (
	"context"
	"sync"

	"golang.org/x/exp/slices"
)

type langsmithTraceOptionKey struct{}

type traceOptions struct {
	SessionName        string
	ReferenceExampleID string
	TraceID            string
	Metadata           *sync.Map
	ParentID           string
	ParentDottedOrder  string
	Tags               []string
}

type TraceOption func(*traceOptions)

// SetTrace 将 trace 选项设置到 context 中
func SetTrace(ctx context.Context, opts ...TraceOption) context.Context {
	options := &traceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, langsmithTraceOptionKey{}, options)
}

// WithSessionName 设置 Langsmith 的项目名称
func WithSessionName(name string) TraceOption {
	return func(o *traceOptions) {
		o.SessionName = name
	}
}

// AddTag 插入tag
func AddTag(tag string) TraceOption {
	return func(o *traceOptions) {
		if o.Tags == nil {
			o.Tags = []string{}
		}
		if !slices.Contains(o.Tags, tag) {
			o.Tags = append(o.Tags, tag)
		}
	}
}

// WithReferenceExampleID 关联到一个 example
func WithReferenceExampleID(id string) TraceOption {
	return func(o *traceOptions) {
		o.ReferenceExampleID = id
	}
}

// WithTraceID 强制指定一个 trace ID
func WithTraceID(id string) TraceOption {
	return func(o *traceOptions) {
		o.TraceID = id
	}
}

// SetMetadata 设置 trace 的元数据, 覆盖写入
func SetMetadata(metadata *sync.Map) TraceOption {
	return func(o *traceOptions) {
		if o.Metadata == nil {
			o.Metadata = metadata
		} else {
			metadata.Range(func(k, v interface{}) bool {
				o.Metadata.Store(k, v)
				return true
			})
		}
	}
}
