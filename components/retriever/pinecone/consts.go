package pinecone

import pc "github.com/pinecone-io/go-pinecone/v3/pinecone"

const (
	typ = "pinecone"

	defaultIndexName  = "eino-index"
	defaultNamespace  = "eino_space"
	defaultField      = "__content__"
	defaultMetricType = pc.Cosine

	defaulttopK = 5
)
