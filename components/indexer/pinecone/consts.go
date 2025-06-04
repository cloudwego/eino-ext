package pinecone

import "github.com/pinecone-io/go-pinecone/v3/pinecone"

const (
	typ                       = "pinecone"
	defaultIndexName          = "eino-index"
	defaultCloud              = pinecone.Aws
	defaultRegion             = "us-east-1"
	defaultVectorType         = "dense"
	defaultDimension          = int32(1536)
	defaultMetric             = pinecone.Cosine
	defaultDeletionProtection = pinecone.DeletionProtectionDisabled
)

const (
	defaultMaxConcurrency = 100
	defaultBatchSize      = 200
)
