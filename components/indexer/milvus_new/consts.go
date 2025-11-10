/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package milvus_new

const (
	typ                           = "MilvusNew"
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
