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

package milvus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// defaultSearchParam returns a default AUTOINDEX search parameter with the given score threshold and radius.
func defaultSearchParam(score float64, dim float64) entity.SearchParam {
	searchParam, _ := entity.NewIndexAUTOINDEXSearchParam(defaultAutoIndexLevel)
	searchParam.AddRadius(dim)
	searchParam.AddRangeFilter(score)
	return searchParam
}

// defaultDocumentConverter returns the default function for converting Milvus search results
// to schema.Document instances. It extracts id, content, and metadata fields from search results.
func defaultDocumentConverter() func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error) {
	return func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error) {
		var err error
		result := make([]*schema.Document, doc.IDs.Len())
		for i := range result {
			result[i] = &schema.Document{
				MetaData: make(map[string]any),
			}
		}
		for _, field := range doc.Fields {
			switch field.Name() {
			case "id":
				for i, document := range result {
					document.ID, err = doc.IDs.GetAsString(i)
					if err != nil {
						return nil, fmt.Errorf("failed to get id: %w", err)
					}
				}
			case "content":
				for i, document := range result {
					document.Content, err = field.GetAsString(i)
					if err != nil {
						return nil, fmt.Errorf("failed to get content: %w", err)
					}
				}
			case "metadata":
				for i, document := range result {
					b, err := field.Get(i)
					bytes, ok := b.([]byte)
					if !ok {
						return nil, fmt.Errorf("failed to get metadata: %w", err)
					}
					if err := sonic.Unmarshal(bytes, &document.MetaData); err != nil {
						return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
					}
				}
			default:
				for i, document := range result {
					document.MetaData[field.Name()], err = field.GetAsString(i)
				}
			}
		}
		return result, nil
	}
}

// defaultVectorConverter returns the default function for converting embedding vectors
// to Milvus BinaryVector format.
func defaultVectorConverter() func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
	return func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
		vec := make([]entity.Vector, 0, len(vectors))
		for _, vector := range vectors {
			vec = append(vec, entity.BinaryVector(vector2Bytes(vector)))
		}
		return vec, nil
	}
}

// checkCollectionSchema verifies that the specified vector field exists in the collection schema.
// It returns an error if the field is not found.
func checkCollectionSchema(field string, s *entity.Schema) error {
	for _, column := range s.Fields {
		if column.Name == field {
			return nil
		}
	}
	return errors.New("vector field not found")
}

// getCollectionDim retrieves the dimension of the specified vector field from the collection schema.
// It returns an error if the field is not found or the dimension cannot be parsed.
func getCollectionDim(field string, s *entity.Schema) (float64, error) {
	for _, column := range s.Fields {
		if column.Name == field {
			scoreCeiling, err := strconv.ParseFloat(column.TypeParams[typeParamDim], 64)
			if err != nil {
				return 0, err
			}
			return scoreCeiling, nil
		}
	}
	return 0, errors.New("vector field not found")
}

// loadCollection ensures the collection is loaded into memory for querying.
// It checks the current load state and waits for loading to complete if in progress.
func loadCollection(ctx context.Context, conf *RetrieverConfig) error {
	loadState, err := conf.Client.GetLoadState(ctx, conf.Collection, nil)
	if err != nil {
		return fmt.Errorf("failed to get load state: %w", err)
	}
	switch loadState {
	case entity.LoadStateNotExist:
		return fmt.Errorf(" collection not exist")
	case entity.LoadStateNotLoad:
		index, err := conf.Client.DescribeIndex(ctx, conf.Collection, conf.VectorField)
		if err != nil {
			if errors.Is(err, client.ErrClientNotReady) {
				return fmt.Errorf(" milvus client not ready: %w", err)
			}
			return err
		}
		if len(index) < 1 {
			return fmt.Errorf("index not found")
		}
		return nil
	case entity.LoadStateLoading:
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				loadingProgress, err := conf.Client.GetLoadingProgress(ctx, conf.Collection, nil)
				if err != nil {
					return err
				}
				if loadingProgress == defaultLoadedProgress {
					return nil
				}
			}
		}
	default:
		return nil
	}
}

// makeEmbeddingCtx creates a context with callback run information for embedding operations.
// This ensures proper tracing and logging of embedding calls within the retriever flow.
func (r *Retriever) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.ReuseHandlers(ctx, runInfo)
}

// vector2Bytes converts a float64 vector to a byte slice representation.
// Each float64 is first converted to float32, then encoded as 4 bytes in little-endian order.
func vector2Bytes(vector []float64) []byte {
	float32Arr := make([]float32, len(vector))
	for i, v := range vector {
		float32Arr[i] = float32(v)
	}
	bytes := make([]byte, len(float32Arr)*4)
	for i, v := range float32Arr {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(v))
	}
	return bytes
}
