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

package milvus

import "github.com/milvus-io/milvus-sdk-go/v2/entity"

const (
	typ                   = "Milvus"
	defaultCollection     = "eino_collection"
	defaultVectorField    = "vector"
	defaultTopK           = 5
	defaultAutoIndexLevel = 1
	defaultLoadedProgress = 100

	defaultMetricType = entity.HAMMING

	typeParamDim = "dim"
)
