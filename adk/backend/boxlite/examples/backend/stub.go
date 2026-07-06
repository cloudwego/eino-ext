//go:build !boxlite

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

package main

import "fmt"

// The real example needs the BoxLite backend, which is behind the "boxlite"
// build tag. This stub keeps the package buildable (and CI green) without it.
func main() {
	fmt.Println("this example requires the 'boxlite' build tag and the BoxLite native library:")
	fmt.Println("  go run github.com/boxlite-ai/boxlite/sdks/go/cmd/setup")
	fmt.Println("  go run -tags boxlite .")
}
