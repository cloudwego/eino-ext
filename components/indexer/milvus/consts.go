package milvus

const (
	typ                           = "Milvus"
	defaultCollection             = "eino_collection"
	defaultDescription            = "the collection for eino"
	defaultCollectionID           = "id"
	defaultCollectionIDDesc       = "the unique id of the document"
	defaultCollectionVector       = "vector"
	defaultCollectionVectorDesc   = "the vector of the document"
	defaultCollectionContent      = "content"
	defaultCollectionContentDesc  = "the content of the document"
	defaultCollectionMetadata     = "metadata"
	defaultCollectionMetadataDesc = "the metadata of the document"

	defaultDim = 81920

	defaultIndexField = "vector"

	defaultConsistencyLevel = ConsistencyLevelBounded
	defaultMetricType       = HAMMING
)
