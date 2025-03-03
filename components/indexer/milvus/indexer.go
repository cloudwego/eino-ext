package milvus

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

type IndexerConfig struct {
	// Client is the milvus client to be called
	// Required
	Client client.Client

	// Default Collection config
	// Collection is the collection name in milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// PartitionNum is the collection partition number
	// Optional, and the default value is 1(disable)
	// If the partition number is larger than 1, it means 启use partition and the partition key is collection id
	PartitionNum int64
	// Description is the description for collection
	// Optional, and the default value is "the collection for eino"
	Description string
	// Dim is the vector dimension
	// Optional, and the default value is 10,240 * 8
	// because the dim it has to be a multiple of 8
	Dim int64
	// SharedNum is the milvus required param to create collection
	// Optional, and the default value is 1
	SharedNum int32
	// ConsistencyLevel is the milvus collection consistency tactics
	// Optional, and the default level is ClBounded(bounded consistency level with default tolerance of 5 seconds)
	ConsistencyLevel ConsistencyLevel
	// EnableDynamicSchema is means the collection is enabled to dynamic schema
	// Optional, and the default value is false
	// Enable to dynamic schema it could affect milvus performance
	EnableDynamicSchema bool

	// Index config to the vector column
	// MetricType the metric type for vector
	// Optional and default type is HAMMING
	// It offers two options: HAMMING and JACCARD
	MetricType MetricType

	// Embedding vectorization method for values needs to be embedded from schema.Document's content.
	// Required
	Embedding embedding.Embedder
}

type Indexer struct {
	config IndexerConfig
}

// NewIndexer creates a new indexer.
func NewIndexer(ctx context.Context, conf *IndexerConfig) (*Indexer, error) {
	// conf check
	if err := conf.check(); err != nil {
		return nil, err
	}

	// check the collection whether to be created
	ok, err := conf.Client.HasCollection(ctx, conf.Collection)
	if err != nil {
		if errors.Is(err, client.ErrClientNotReady) {
			return nil, fmt.Errorf("[NewIndexer] milvus client not ready: %w", err)
		}
		if errors.Is(err, client.ErrStatusNil) {
			return nil, fmt.Errorf("[NewIndexer] milvus client status is nil: %w", err)
		}
		return nil, fmt.Errorf("[NewIndexer] failed to check collection: %w", err)
	}
	if !ok {
		// create the collection
		isPartition := false
		if conf.PartitionNum > 1 {
			isPartition = false
		}
		if errToCreate := conf.Client.CreateCollection(ctx,
			getDefaultSchema(conf.Collection, conf.Description, isPartition, conf.Dim),
			conf.SharedNum,
			client.WithConsistencyLevel(
				conf.ConsistencyLevel.getConsistencyLevel(),
			),
			client.WithEnableDynamicSchema(conf.EnableDynamicSchema),
			client.WithPartitionNum(conf.PartitionNum),
		); errToCreate != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to create collection: %w", errToCreate)
		}
	}

	// load collection info
	collection, err := conf.Client.DescribeCollection(ctx, conf.Collection)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to describe collection: %w", err)
	}
	// check collection schema
	if !checkCollectionSchema(collection.Schema) {
		return nil, fmt.Errorf("[NewIndexer] collection schema not match")
	}
	// check the collection load state
	if !collection.Loaded {
		// load collection
		if err := loadCollection(ctx, conf); err != nil {
			return nil, err
		}
	}

	// create indexer
	return &Indexer{
		config: *conf,
	}, nil
}

// Store stores the documents into the indexer.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// get common options
	co := indexer.GetCommonOptions(&indexer.Options{
		SubIndexes: nil,
		Embedding:  i.config.Embedding,
	}, opts...)

	// callback info on start
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{
		Docs: docs,
	})

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[Indexer.Store] embedding not provided")
	}

	// load documents content
	em := make([]defaultSchema, 0, len(docs))
	texts := make([]string, 0, len(docs))
	rows := make([]interface{}, 0, len(docs))
	for _, doc := range docs {
		metadata, err := sonic.Marshal(doc.MetaData)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to marshal metadata: %w", err)
		}
		em = append(em, defaultSchema{
			ID:       doc.ID,
			Content:  doc.Content,
			Vector:   nil,
			Metadata: metadata,
		})
		texts = append(texts, doc.Content)
	}

	// embedding
	vector, err := co.Embedding.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, err
	}
	if len(vector) != len(docs) {
		return nil, fmt.Errorf("embedding result length not match")
	}

	// build embedding documents for storing
	for idx, vec := range vector {
		em[idx].Vector = vector2Bytes(vec)
		rows = append(rows, &em[idx])
	}

	// store documents into milvus
	results, err := i.config.Client.InsertRows(ctx, i.config.Collection, "", rows)
	if err != nil {
		return nil, err
	}

	// flush collection to make sure the data is visible
	if err := i.config.Client.Flush(ctx, i.config.Collection, false); err != nil {
		return nil, err
	}

	// callback info on end
	ids = make([]string, results.Len())
	for idx := 0; idx < results.Len(); idx++ {
		ids[idx], err = results.GetAsString(idx)
		if err != nil {
			return nil, err
		}
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{
		IDs: ids,
	})
	return ids, nil
}

// check the indexer config
func (i *IndexerConfig) check() error {
	if i.Client == nil {
		return fmt.Errorf("[NewIndexer] milvus client not provided")
	}
	if i.Embedding == nil {
		return fmt.Errorf("[NewIndexer] embedding not provided")
	}
	if i.Dim <= 0 {
		i.Dim = defaultDim
	}
	if i.Dim%8 != 0 {
		return fmt.Errorf("[NewIndexer] invalid dim")
	}
	if i.Collection == "" {
		i.Collection = defaultCollection
	}
	if i.Description == "" {
		i.Description = defaultDescription
	}
	if i.SharedNum <= 0 {
		i.SharedNum = 1
	}
	if i.ConsistencyLevel <= 0 || i.ConsistencyLevel > 5 {
		i.ConsistencyLevel = defaultConsistencyLevel
	}
	if i.MetricType == "" {
		i.MetricType = defaultMetricType
	}
	if i.PartitionNum <= 1 {
		i.PartitionNum = 0
	}
	return nil
}

// makeEmbeddingCtx makes the embedding context.
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
