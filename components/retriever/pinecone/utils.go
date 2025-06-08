package pinecone

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
)

// makeEmbeddingCtx makes the embedding context
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

func defaultVectorConverter() func(ctx context.Context, vector []float64) ([]float32, error) {
	return func(ctx context.Context, vector []float64) ([]float32, error) {
		vec := make([]float32, 0, len(vector))
		for _, value := range vector {
			vec = append(vec, float32(value))
		}
		return vec, nil
	}
}

func defaultDocumentConverter() func(ctx context.Context, vector *pc.Vector, field string) (*schema.Document, error) {
	return func(ctx context.Context, vector *pc.Vector, field string) (*schema.Document, error) {
		data := vector.Metadata.AsMap()
		if _, exists := data[field]; !exists {
			return nil, fmt.Errorf("[converter] content field not found, field: %s", field)
		}
		content := data[field].(string)
		meta := make(map[string]any)
		for k, v := range data {
			if k != field {
				meta[k] = v
			}
		}
		return &schema.Document{
			ID:       vector.Id,
			Content:  content,
			MetaData: meta,
		}, nil
	}
}
