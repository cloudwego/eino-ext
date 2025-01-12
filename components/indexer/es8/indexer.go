package es8

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/cloudwego/eino-ext/components/indexer/es8/field_mapping"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

type IndexerConfig struct {
	ESConfig  elasticsearch.Config `json:"es_config"`
	Index     string               `json:"index"`
	BatchSize int                  `json:"batch_size"`

	// VectorFields dense_vector field mappings
	VectorFields []field_mapping.FieldKV `json:"vector_fields"`
	// Embedding vectorization method, must provide in two cases
	// 1. VectorFields contains fields except doc Content
	// 2. VectorFields contains doc Content and vector not provided in doc extra (see Document.Vector method)
	Embedding embedding.Embedder
}

type Indexer struct {
	client *elasticsearch.Client
	config *IndexerConfig
}

func NewIndexer(_ context.Context, conf *IndexerConfig) (*Indexer, error) {
	client, err := elasticsearch.NewClient(conf.ESConfig)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] new es client failed, %w", err)
	}

	if conf.Embedding == nil {
		for _, kv := range conf.VectorFields {
			if kv.FieldName != field_mapping.DocFieldNameContent {
				return nil, fmt.Errorf("[NewIndexer] Embedding not provided in config, but field kv[%s]-[%s] requires",
					kv.FieldNameVector, kv.FieldName)
			}
		}
	}

	if conf.BatchSize == 0 {
		conf.BatchSize = defaultBatchSize
	}

	return &Indexer{
		client: client,
		config: conf,
	}, nil
}

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})

	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:  i.config.Index,
		Client: i.client,
	})
	if err != nil {
		return nil, err
	}

	for _, slice := range chunk(docs, i.config.BatchSize) {
		var items []esutil.BulkIndexerItem

		if len(i.config.VectorFields) == 0 {
			items, err = i.defaultQueryItems(ctx, slice, options)
		} else {
			items, err = i.vectorQueryItems(ctx, slice, options)
		}
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			if err = bi.Add(ctx, item); err != nil {
				return nil, err
			}
		}
	}

	if err = bi.Close(ctx); err != nil {
		return nil, err
	}

	ids = iter(docs, func(t *schema.Document) string { return t.ID })

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) defaultQueryItems(_ context.Context, docs []*schema.Document, _ *indexer.Options) (items []esutil.BulkIndexerItem, err error) {
	items, err = iterWithErr(docs, func(doc *schema.Document) (item esutil.BulkIndexerItem, err error) {
		b, err := json.Marshal(toESDoc(doc))
		if err != nil {
			return item, err
		}

		return esutil.BulkIndexerItem{
			Index:      i.config.Index,
			Action:     "index",
			DocumentID: doc.ID,
			Body:       bytes.NewReader(b),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}

func (i *Indexer) vectorQueryItems(ctx context.Context, docs []*schema.Document, options *indexer.Options) (items []esutil.BulkIndexerItem, err error) {
	emb := options.Embedding

	items, err = iterWithErr(docs, func(doc *schema.Document) (item esutil.BulkIndexerItem, err error) {
		mp := toESDoc(doc)
		texts := make([]string, 0, len(i.config.VectorFields))
		for _, kv := range i.config.VectorFields {
			str, ok := kv.FieldName.Find(doc)
			if !ok {
				return item, fmt.Errorf("[vectorQueryItems] field name not found or type incorrect, name=%s, doc=%v", kv.FieldName, doc)
			}

			if kv.FieldName == field_mapping.DocFieldNameContent && len(doc.Vector()) > 0 {
				mp[string(kv.FieldNameVector)] = doc.Vector()
			} else {
				texts = append(texts, str)
			}
		}

		if len(texts) > 0 {
			if emb == nil {
				return item, fmt.Errorf("[vectorQueryItems] embedding not provided")
			}

			vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
			if err != nil {
				return item, fmt.Errorf("[vectorQueryItems] embedding failed, %w", err)
			}

			if len(vectors) != len(texts) {
				return item, fmt.Errorf("[vectorQueryItems] invalid vector length, expected=%d, got=%d", len(texts), len(vectors))
			}

			vIdx := 0
			for _, kv := range i.config.VectorFields {
				if kv.FieldName == field_mapping.DocFieldNameContent && len(doc.Vector()) > 0 {
					continue
				}

				mp[string(kv.FieldNameVector)] = vectors[vIdx]
				vIdx++
			}
		}

		b, err := json.Marshal(mp)
		if err != nil {
			return item, err
		}

		return esutil.BulkIndexerItem{
			Index:      i.config.Index,
			Action:     "index",
			DocumentID: doc.ID,
			Body:       bytes.NewReader(b),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}

func (i *Indexer) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.SwitchRunInfo(ctx, runInfo)
}

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}
