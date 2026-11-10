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

package qdrant

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

// VectorConfig describes one named vector space of a Qdrant collection.
//
// Qdrant keeps the named vectors of a point independently, but an upsert
// replaces the whole point: every vector that is not part of the request is
// set to null. Named vector spaces can therefore only coexist if they are
// written by the same upsert, which is what Config.Vectors arranges.
type VectorConfig struct {
	// Name is the name of the vector space in Qdrant. Required and must be
	// unique within Config.Vectors.
	Name string
	// Dimension is the length of the vectors produced by Embedding. Required.
	Dimension int
	// Distance is the distance metric of this vector space. Required: the zero
	// value is Distance_UnknownDistance, which is rejected here rather than by
	// the server.
	Distance qdrant.Distance
	// Embedding generates the vectors written to this space. Required.
	Embedding embedding.Embedder
}

type Config struct {
	// Qdrant gRPC client
	Client *qdrant.Client
	// Collection name
	Collection string
	// Vector dimension of the single unnamed vector. Used when Vectors is empty.
	VectorDim int
	// Distance metric of the single unnamed vector. Used when Vectors is empty.
	Distance qdrant.Distance
	// BatchSize controls embedding texts size.
	BatchSize int
	// Embedder used to generate vector representations for documents.
	// Used when Vectors is empty.
	Embedding embedding.Embedder
	// Vectors configures one or more named vector spaces (e.g. one per
	// embedding model). Each Store call then computes every configured space
	// and writes all of them in a single upsert, which is the only way for
	// several spaces to coexist in one collection.
	//
	// Vectors is mutually exclusive with VectorDim, Distance and Embedding,
	// which describe the unnamed single-vector layout; each space carries its
	// own dimension and distance instead. Note that Qdrant still nulls any
	// vector space that is missing from the request, so a point written
	// through this indexer drops the vector spaces it does not configure -
	// including spaces created outside this indexer.
	Vectors []VectorConfig
}

type Indexer struct {
	client     *qdrant.Client
	collection string
	vectorDim  int
	distance   qdrant.Distance
	vectors    []VectorConfig
	batchSize  int
	embedding  embedding.Embedder
}

func NewIndexer(ctx context.Context, config *Config) (*Indexer, error) {
	if len(config.Vectors) == 0 && config.Embedding == nil {
		return nil, fmt.Errorf("[NewIndexer] embedding not provided for qdrant indexer")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("[NewIndexer] qdrant client not provided")
	}
	if err := validateVectors(config); err != nil {
		return nil, err
	}

	collection := config.Collection
	if collection == "" {
		collection = defaultCollection
	}

	batchSize := config.BatchSize
	if batchSize == 0 {
		batchSize = 10
	}

	indexer := &Indexer{
		client:     config.Client,
		collection: collection,
		vectorDim:  config.VectorDim,
		distance:   config.Distance,
		vectors:    config.Vectors,
		batchSize:  batchSize,
		embedding:  config.Embedding,
	}

	if err := indexer.ensureCollection(ctx); err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to ensure collection: %w", err)
	}

	return indexer, nil
}

// validateVectors checks the multi-vector configuration, which is mutually
// exclusive with the unnamed single-vector fields.
func validateVectors(config *Config) error {
	if len(config.Vectors) == 0 {
		return nil
	}
	if config.VectorDim != 0 || config.Distance != 0 || config.Embedding != nil {
		return fmt.Errorf("[NewIndexer] Vectors cannot be combined with VectorDim, Distance or Embedding; " +
			"configure the dimension, the distance and the embedder on each VectorConfig instead")
	}

	seen := make(map[string]struct{}, len(config.Vectors))
	for idx, v := range config.Vectors {
		if v.Name == "" {
			return fmt.Errorf("[NewIndexer] Vectors[%d]: Name is required", idx)
		}
		if _, ok := seen[v.Name]; ok {
			return fmt.Errorf("[NewIndexer] Vectors[%d]: duplicate Name %q", idx, v.Name)
		}
		seen[v.Name] = struct{}{}

		if v.Dimension <= 0 {
			return fmt.Errorf("[NewIndexer] Vectors[%d] (%q): Dimension must be positive, got %d",
				idx, v.Name, v.Dimension)
		}
		if v.Distance == qdrant.Distance_UnknownDistance {
			return fmt.Errorf("[NewIndexer] Vectors[%d] (%q): Distance is required", idx, v.Name)
		}
		if v.Embedding == nil {
			return fmt.Errorf("[NewIndexer] Vectors[%d] (%q): Embedding is required", idx, v.Name)
		}
	}
	return nil
}

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if err = i.batchUpsert(ctx, docs, options); err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})
	return ids, nil
}

func (i *Indexer) batchUpsert(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	batchSize := i.batchSize

	if len(i.vectors) > 0 && options.Embedding != nil {
		return fmt.Errorf("[batchUpsert] embedding option is not supported when Config.Vectors is set, " +
			"configure Embedding on each VectorConfig instead")
	}

	for start := 0; start < len(docs); start += batchSize {
		end := start + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]
		texts := make([]string, 0, len(batch))
		for _, doc := range batch {
			texts = append(texts, doc.Content)
		}

		// vectors holds, for every document of the batch, the vector of each
		// configured space keyed by the space name. The unnamed mode uses the
		// empty string as its single key.
		vectors, err := i.embedBatch(ctx, texts, options)
		if err != nil {
			return err
		}

		points := make([]*qdrant.PointStruct, 0, len(batch))
		for idx, doc := range batch {
			point := &qdrant.PointStruct{
				Id:      qdrant.NewID(doc.ID),
				Vectors: i.buildVectors(vectors[idx]),
				Payload: qdrant.NewValueMap(map[string]any{
					defaultContentKey:  doc.Content,
					defaultMetadataKey: doc.MetaData,
				}),
			}
			points = append(points, point)
		}
		_, err = i.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: i.collection,
			Points:         points,
		})
		if err != nil {
			return fmt.Errorf("[batchUpsert] failed to upsert points to qdrant, %w", err)
		}
	}
	return nil
}

// embedBatch computes, for each text of the batch, the vectors of every
// configured space. All spaces are computed before the upsert, so a single
// request can carry them together - writing one space at a time would null the
// others.
func (i *Indexer) embedBatch(ctx context.Context, texts []string, options *indexer.Options) ([]map[string][]float32, error) {
	if len(i.vectors) == 0 {
		return i.embedUnnamed(ctx, texts, options.Embedding)
	}

	vectors := make([]map[string][]float32, len(texts))
	for idx := range vectors {
		vectors[idx] = make(map[string][]float32, len(i.vectors))
	}

	for _, space := range i.vectors {
		emb := space.Embedding
		embedded, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return nil, fmt.Errorf("[batchUpsert] embedding failed for vector %q, %w", space.Name, err)
		}
		if len(embedded) != len(texts) {
			return nil, fmt.Errorf("[batchUpsert] invalid vector count for vector %q, expected=%d, got=%d",
				space.Name, len(texts), len(embedded))
		}
		for idx, vec := range embedded {
			if len(vec) != space.Dimension {
				return nil, fmt.Errorf("[batchUpsert] vector %q: expected dimension %d, got %d",
					space.Name, space.Dimension, len(vec))
			}
			vectors[idx][space.Name] = float64SliceToFloat32(vec)
		}
	}
	return vectors, nil
}

func (i *Indexer) embedUnnamed(ctx context.Context, texts []string, emb embedding.Embedder) ([]map[string][]float32, error) {
	embedded, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, fmt.Errorf("[batchUpsert] embedding failed, %w", err)
	}
	if len(embedded) != len(texts) {
		return nil, fmt.Errorf("[batchUpsert] invalid vector length, expected=%d, got=%d", len(texts), len(embedded))
	}

	vectors := make([]map[string][]float32, len(texts))
	for idx, vec := range embedded {
		vectors[idx] = map[string][]float32{"": float64SliceToFloat32(vec)}
	}
	return vectors, nil
}

func (i *Indexer) buildVectors(vectors map[string][]float32) *qdrant.Vectors {
	if len(i.vectors) == 0 {
		return qdrant.NewVectors(vectors[""]...)
	}

	named := make(map[string]*qdrant.Vector, len(vectors))
	for name, vec := range vectors {
		named[name] = qdrant.NewVector(vec...)
	}
	return qdrant.NewVectorsMap(named)
}

func (i *Indexer) ensureCollection(ctx context.Context) error {
	exists, err := i.client.CollectionExists(ctx, i.collection)
	if err != nil {
		return err
	}

	if exists {
		if len(i.vectors) == 0 {
			return nil
		}
		return i.checkVectors(ctx)
	}

	var vectorsConfig *qdrant.VectorsConfig
	if len(i.vectors) > 0 {
		params := make(map[string]*qdrant.VectorParams, len(i.vectors))
		for _, v := range i.vectors {
			params[v.Name] = &qdrant.VectorParams{
				Size:     uint64(v.Dimension),
				Distance: v.Distance,
			}
		}
		vectorsConfig = qdrant.NewVectorsConfigMap(params)
	} else {
		vectorsConfig = qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(i.vectorDim),
			Distance: i.distance,
		})
	}

	err = i.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: i.collection,
		VectorsConfig:  vectorsConfig,
	})
	return err
}

// checkVectors verifies that an existing collection carries every configured
// named vector with the expected dimension and distance.
//
// A missing named vector cannot be added here: in go-client v1.15.x
// UpdateCollection.vectors_config is a VectorsConfigDiff, whose
// VectorParamsDiff carries no size/distance, so the gRPC API cannot introduce a
// new named vector with a dimension. Qdrant itself supports it since v1.18
// (PUT /collections/{collection_name}/vectors/{vector_name}), but that endpoint has no
// counterpart in the client used by this module.
func (i *Indexer) checkVectors(ctx context.Context) error {
	info, err := i.client.GetCollectionInfo(ctx, i.collection)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to get collection info: %w", err)
	}

	existing := info.GetConfig().GetParams().GetVectorsConfig().GetParamsMap().GetMap()
	for _, v := range i.vectors {
		params, ok := existing[v.Name]
		if !ok {
			return fmt.Errorf(
				"[NewIndexer] collection %q already exists but does not carry a named vector %q; "+
					"named vectors must be created together with the collection, because the gRPC API "+
					"of the Qdrant client used here (go-client v1.15.x, VectorsConfigDiff) cannot add a "+
					"named vector with a dimension to an existing collection",
				i.collection, v.Name)
		}
		if params.GetSize() != uint64(v.Dimension) {
			return fmt.Errorf(
				"[NewIndexer] collection %q has named vector %q with size %d, but Dimension is %d",
				i.collection, v.Name, params.GetSize(), v.Dimension)
		}
		if params.GetDistance() != v.Distance {
			return fmt.Errorf(
				"[NewIndexer] collection %q has named vector %q with distance %s, but Distance is %s",
				i.collection, v.Name, params.GetDistance(), v.Distance)
		}
	}
	return nil
}

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

func float64SliceToFloat32(v []float64) []float32 {
	f := make([]float32, len(v))
	for i, x := range v {
		f[i] = float32(x)
	}
	return f
}

// makeEmbeddingCtx creates a context with embedding callback information.
func (i *Indexer) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.ReuseHandlers(ctx, runInfo)
}
