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
	"context"
	
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

func makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}
	
	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}
	
	runInfo.Name = runInfo.Type + string(runInfo.Component)
	
	return callbacks.ReuseHandlers(ctx, runInfo)
}

func vector2Float32(vector []float64) []float32 {
	float32Arr := make([]float32, len(vector))
	for i, v := range vector {
		float32Arr[i] = float32(v)
	}
	return float32Arr
}
