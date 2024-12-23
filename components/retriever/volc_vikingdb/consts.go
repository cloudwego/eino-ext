/*
 * Copyright 2024 CloudWeGo Authors
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

package volc_vikingdb

const typ = "VikingDB"

const (
	ExtraKeyVikingDBFields = "_vikingdb_fields" // value: map[string]interface{}
	ExtraKeyVikingDBTTL    = "_vikingdb_ttl"    // value: int64
)

const (
	defaultFieldContent = "content"
)

const (
	vikingEmbeddingUseDense           = "return_dense"
	vikingEmbeddingUseSparse          = "return_sparse"
	vikingEmbeddingRespSentenceDense  = "sentence_dense_embedding"
	vikingEmbeddingRespSentenceSparse = "sentence_sparse_embedding"
)
