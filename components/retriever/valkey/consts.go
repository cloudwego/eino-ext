/*
 * Copyright 2026 CloudWeGo Authors
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

package valkey

const (
	defaultReturnFieldContent       = "content"
	defaultReturnFieldVectorContent = "vector_content"
	paramVector                     = "vector"
	paramDistanceThreshold          = "distance_threshold"
	// SortByDistanceAttributeName is the alias assigned to the computed distance
	// in KNN vector search queries (via "AS distance" in the query expression).
	// If a document field has this same name, the distance value will shadow it in
	// search results, causing incorrect data in the returned documents. To avoid
	// collisions, rename the document field or use a custom DocumentConverter.
	SortByDistanceAttributeName = "distance"
)
