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

package milvus_new

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
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// defaultDocumentConverter returns the default document converter
func defaultDocumentConverter() func(ctx context.Context, columns []column.Column) ([]*schema.Document, error) {
	return func(ctx context.Context, columns []column.Column) ([]*schema.Document, error) {
		if len(columns) == 0 {
			return nil, nil
		}

		// Determine the number of documents from the first column
		numDocs := columns[0].Len()
		result := make([]*schema.Document, numDocs)
		for i := range result {
			result[i] = &schema.Document{
				MetaData: make(map[string]any),
			}
		}

		// Process each column
		for _, col := range columns {
			switch col.Name() {
			case "id":
				for i := 0; i < col.Len(); i++ {
					val, err := col.Get(i)
					if err != nil {
						return nil, fmt.Errorf("failed to get id: %w", err)
					}
					if str, ok := val.(string); ok {
						result[i].ID = str
					}
				}
			case "content":
				for i := 0; i < col.Len(); i++ {
					val, err := col.Get(i)
					if err != nil {
						return nil, fmt.Errorf("failed to get content: %w", err)
					}
					if str, ok := val.(string); ok {
						result[i].Content = str
					}
				}
			case "metadata":
				for i := 0; i < col.Len(); i++ {
					val, err := col.Get(i)
					if err != nil {
						return nil, fmt.Errorf("failed to get metadata: %w", err)
					}
					if bytes, ok := val.([]byte); ok {
						if err := sonic.Unmarshal(bytes, &result[i].MetaData); err != nil {
							return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
						}
					}
				}
			default:
				// Add other fields to metadata
				for i := 0; i < col.Len(); i++ {
					val, err := col.Get(i)
					if err != nil {
						continue
					}
					result[i].MetaData[col.Name()] = val
				}
			}
		}

		return result, nil
	}
}

// defaultVectorConverter returns the default vector converter
func defaultVectorConverter() func(ctx context.Context, vectors [][]float64) ([][]byte, error) {
	return func(ctx context.Context, vectors [][]float64) ([][]byte, error) {
		result := make([][]byte, 0, len(vectors))
		for _, vector := range vectors {
			result = append(result, vector2Bytes(vector))
		}
		return result, nil
	}
}

// checkCollectionSchema checks if the vector field exists in the schema
func checkCollectionSchema(field string, s *entity.Schema) error {
	for _, column := range s.Fields {
		if column.Name == field {
			return nil
		}
	}
	return errors.New("vector field not found")
}

// getCollectionDim gets the dimension of the vector field
func getCollectionDim(field string, s *entity.Schema) (int, error) {
	for _, column := range s.Fields {
		if column.Name == field {
			dimStr, ok := column.TypeParams[typeParamDim]
			if !ok {
				return 0, errors.New("dim not found in type params")
			}
			dim, err := strconv.Atoi(dimStr)
			if err != nil {
				return 0, fmt.Errorf("failed to parse dim: %w", err)
			}
			return dim, nil
		}
	}
	return 0, errors.New("vector field not found")
}

// loadCollection loads the collection
func loadCollection(ctx context.Context, client milvusclient.Client, collectionName string) error {
	loadStateOpt := milvusclient.NewGetLoadStateOption(collectionName)
	loadState, err := client.GetLoadState(ctx, loadStateOpt)
	if err != nil {
		return fmt.Errorf("failed to get load state: %w", err)
	}

	switch loadState.State {
	case entity.LoadStateNotLoad:
		return fmt.Errorf("collection not loaded")
	case entity.LoadStateLoading:
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				state, err := client.GetLoadState(ctx, loadStateOpt)
				if err != nil {
					return err
				}
				if state.State == entity.LoadStateLoaded {
					return nil
				}
			}
		}
	default:
		return nil
	}
}

// makeEmbeddingCtx makes the embedding context
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

// vector2Bytes converts the vector to bytes
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
