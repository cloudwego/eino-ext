package milvus

import "github.com/milvus-io/milvus-sdk-go/v2/entity"

const (
	HAMMING = MetricType(entity.HAMMING)
	JACCARD = MetricType(entity.JACCARD)
)

// defaultSchema is the default schema for milvus by eino
type defaultSchema struct {
	ID       string `json:"id" milvus:"name:id"`
	Content  string `json:"content" milvus:"name:content"`
	Vector   []byte `json:"vector" milvus:"name:vector"`
	Metadata []byte `json:"metadata" milvus:"name:metadata"`
}

// MetricType is the metric type for vector by eino
type MetricType entity.MetricType

// getMetricType returns the metric type
func (t *MetricType) getMetricType() entity.MetricType {
	return entity.MetricType(*t)
}
