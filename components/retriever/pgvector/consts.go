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

package pgvector

import (
	"fmt"
)

// DistanceFunction represents the distance function for vector similarity search.
type DistanceFunction string

const (
	// DistanceCosine uses cosine distance for similarity.
	DistanceCosine DistanceFunction = "cosine"
	// DistanceL2 uses Euclidean (L2) distance.
	DistanceL2 DistanceFunction = "l2"
	// DistanceIP uses inner product distance.
	DistanceIP DistanceFunction = "ip"
)

// String returns the string representation of the distance function.
func (d DistanceFunction) String() string {
	return string(d)
}

// Operator returns the SQL operator for the distance function.
func (d DistanceFunction) Operator() string {
	switch d {
	case DistanceCosine:
		return "<=>"
	case DistanceL2:
		return "<->"
	case DistanceIP:
		return "<#>"
	default:
		return "<=>"
	}
}

// Validate checks if the distance function is valid.
func (d DistanceFunction) Validate() error {
	switch d {
	case DistanceCosine, DistanceL2, DistanceIP:
		return nil
	default:
		return fmt.Errorf("invalid distance function: %s", d)
	}
}

const (
	// DefaultTableName is the default table name for storing documents.
	DefaultTableName = "documents"
)
