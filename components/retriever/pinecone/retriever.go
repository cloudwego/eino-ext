package pinecone

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
)

type RetrieverConfig struct {
	Client *pc.Client

	IndexName  string
	Namespace  string
	MetricType pc.IndexMetric
	Field      string

	VectorConverter   func(ctx context.Context, vector []float64) ([]float32, error)
	DocumentConverter func(ctx context.Context, vector *pc.Vector, field string) (*schema.Document, error)

	TopK           int
	ScoreThreshold float64

	Embedding embedding.Embedder
}

type Retriever struct {
	config RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if err := config.check(); err != nil {
		return nil, err
	}

	index, err := config.Client.DescribeIndex(ctx, config.IndexName)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to describe index: %w", err)
	}

	if err := config.validateIndex(index); err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to validate index, err: %w", err)
	}

	return &Retriever{
		config: *config,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	// get common options
	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.IndexName,
		TopK:           &r.config.TopK,
		ScoreThreshold: &r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	// callback info on start
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		ScoreThreshold: co.ScoreThreshold,
		Extra: map[string]any{
			"metric_type": r.config.MetricType,
		},
	})
	// callback on error
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// get the embedding vector
	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[pinecone retriever] embedding not provided")
	}

	embeddingQuery, err := r.embeddingQuery(ctx, emb, query)
	if err != nil {
		return nil, fmt.Errorf("[pinecone retriever] failed to embedding query, err: %w", err)
	}

	queryVec, err := r.config.VectorConverter(ctx, embeddingQuery)
	if err != nil {
		return nil, fmt.Errorf("[pinecone retriever] failed to convert vector: %w", err)
	}

	// search on pinecone index
	index, err := r.config.Client.DescribeIndex(ctx, r.config.IndexName)
	if err != nil {
		return nil, fmt.Errorf("[pinecone retriever] failed to describe index, err: %w", err)
	}
	indexConn, err := r.config.Client.Index(pc.NewIndexConnParams{
		Host:      index.Host,
		Namespace: r.config.Namespace,
	})
	if err != nil {
		log.Fatalf("[pinecone retriever] failed to create IndexConnection for Host: %v", err)
	}
	pcResp, err := indexConn.QueryByVectorValues(ctx, &pc.QueryByVectorValuesRequest{
		Vector:          queryVec,
		TopK:            uint32(r.config.TopK),
		IncludeValues:   true,
		IncludeMetadata: true,
	})
	if err != nil {
		log.Fatalf("[pinecone retriever] error encountered when querying by vector: %v", err)
	}
	// check the search result
	if len(pcResp.Matches) == 0 {
		return nil, fmt.Errorf("[pinecone retriever] no results found")
	}

	// convert the search result to schema.Document
	documents := make([]*schema.Document, 0, len(pcResp.Matches))
	for _, record := range pcResp.Matches {
		if co.ScoreThreshold != nil && float64(record.Score) < *co.ScoreThreshold {
			continue
		}
		document, err := r.config.DocumentConverter(ctx, record.Vector, r.config.Field)
		if err != nil {
			return nil, fmt.Errorf("[pinecone retriever] failed to convert search result to schema.Document: %w", err)
		}
		documents = append(documents, document)
	}

	// callback info on end
	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: documents})

	return documents, nil
}

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

func (r *Retriever) embeddingQuery(ctx context.Context, embedder embedding.Embedder, query string) ([]float64, error) {
	// embedding the query
	vectors, err := embedder.EmbedStrings(r.makeEmbeddingCtx(ctx, embedder), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[pinecone retriever] embedding has error: %w", err)
	}

	// check the embedding result
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[pinecone retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	return vectors[0], nil
}

func (rc *RetrieverConfig) validateIndex(index *pc.Index) error {
	if index.Metric != rc.MetricType {
		return fmt.Errorf("[validate] index metric and config metric mismatch, index: %s, config: %s", index.Metric, rc.MetricType)
	}
	return nil
}

func (rc *RetrieverConfig) check() error {
	if rc.Client == nil {
		return fmt.Errorf("[NewRetriever] milvus client not provided")
	}
	if rc.Embedding == nil {
		return fmt.Errorf("[NewRetriever] embedding not provided")
	}
	if rc.ScoreThreshold < 0 {
		return fmt.Errorf("[NewRetriever] invalid search params")
	}
	if rc.IndexName == "" {
		rc.IndexName = defaultIndexName
	}
	if rc.Namespace == "" {
		rc.Namespace = defaultNamespace
	}
	if rc.MetricType == "" {
		rc.MetricType = defaultMetricType
	}
	if rc.Field == "" {
		rc.Field = defaultField
	}
	if rc.TopK == 0 {
		rc.TopK = defaulttopK
	}
	if rc.VectorConverter == nil {
		rc.VectorConverter = defaultVectorConverter()
	}
	if rc.DocumentConverter == nil {
		rc.DocumentConverter = defaultDocumentConverter()
	}
	return nil
}
