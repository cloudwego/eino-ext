package milvus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// getDefaultSchema returns the default schema
func getDefaultSchema(collection, description string, isPartition bool, dim int64) *entity.Schema {
	return entity.NewSchema().
		WithName(collection).
		WithDescription(description).
		WithField(
			entity.NewField().
				WithName(defaultCollectionID).
				WithDescription(defaultCollectionIDDesc).
				WithIsPrimaryKey(true).
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(255).
				WithIsPartitionKey(isPartition),
		).
		WithField(
			entity.NewField().
				WithName(defaultCollectionVector).
				WithDescription(defaultCollectionVectorDesc).
				WithIsPrimaryKey(false).
				WithDataType(entity.FieldTypeBinaryVector).
				WithDim(dim),
		).
		WithField(
			entity.NewField().
				WithName(defaultCollectionContent).
				WithDescription(defaultCollectionContentDesc).
				WithIsPrimaryKey(false).
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(1024),
		).
		WithField(
			entity.NewField().
				WithName(defaultCollectionMetadata).
				WithDescription(defaultCollectionMetadataDesc).
				WithIsPrimaryKey(false).
				WithDataType(entity.FieldTypeJSON),
		)
}

// createdDefaultIndex creates the default index
func createdDefaultIndex(ctx context.Context, conf *IndexerConfig, async bool) error {
	index, err := entity.NewIndexAUTOINDEX(conf.MetricType.getMetricType())
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}
	if err := conf.Client.CreateIndex(ctx, conf.Collection, "vector", index, async); err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}
	return nil
}

// checkCollectionSchema checks the collection schema
func checkCollectionSchema(s *entity.Schema) bool {
	var idType entity.FieldType
	var vectorType entity.FieldType
	var contentType entity.FieldType
	var metadataType entity.FieldType
	for _, field := range s.Fields {
		switch field.Name {
		case defaultCollectionID:
			idType = field.DataType
		case defaultCollectionVector:
			vectorType = field.DataType
		case defaultCollectionContent:
			contentType = field.DataType
		case defaultCollectionMetadata:
			metadataType = field.DataType
		default:
			continue
		}
	}
	if idType != entity.FieldTypeVarChar || vectorType != entity.FieldTypeBinaryVector || contentType != entity.FieldTypeVarChar || metadataType != entity.FieldTypeJSON {
		return false
	}
	return true
}

// getCollectionDim gets the collection dimension
func loadCollection(ctx context.Context, conf *IndexerConfig) error {
	loadState, err := conf.Client.GetLoadState(ctx, conf.Collection, nil)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to get load state: %w", err)
	}
	switch loadState {
	case entity.LoadStateNotExist:
		return fmt.Errorf("[NewIndexer] collection not exist")
	case entity.LoadStateNotLoad:
		index, err := conf.Client.DescribeIndex(ctx, conf.Collection, "vector")
		if errors.Is(err, client.ErrClientNotReady) {
			return fmt.Errorf("[NewIndexer] milvus client not ready: %w", err)
		}
		if len(index) == 0 {
			if err := createdDefaultIndex(ctx, conf, false); err != nil {
				return err
			}
		}
		if err := conf.Client.LoadCollection(ctx, conf.Collection, true); err != nil {
			return err
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
				if loadingProgress == 100 {
					return nil
				}
			}
		}
	default:
		return nil
	}
}

// vector2Bytes converts vector to bytes
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
