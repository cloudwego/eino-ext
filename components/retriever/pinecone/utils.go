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

package pinecone

import "encoding/json"

func f64To32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, f := range f64 {
		f32[i] = float32(f)
	}

	return f32
}

func f32To64(f32 []float32) []float64 {
	f64 := make([]float64, len(f32))
	for i, f := range f32 {
		f64[i] = float64(f)
	}

	return f64
}

func marshalStringNoErr(in any) string {
	if in == nil {
		return ""
	}

	b, _ := json.Marshal(in)
	return string(b)
}
