package milvus

import "github.com/milvus-io/milvus-sdk-go/v2/entity"

const (
	defaultCollection     = "eino_collection"
	defaultVectorField    = "vector"
	defaultTopK           = 5
	defaultAutoIndexLevel = 1
	defaultLoadedProgress = 100

	defaultMetricType = entity.HAMMING

	typeParamDim = "dim"
)
