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

package ark

import (
	"fmt"
	"strconv"
)

// Preserve the decimal JSON representation when widening a float32 setting.
func float64FromFloat32(value float32) float64 {
	result, _ := strconv.ParseFloat(strconv.FormatFloat(float64(value), 'g', -1, 32), 64)
	return result
}

func newPanicErr(info any, stack []byte) error {
	return fmt.Errorf("panic error: %v, stack: %s", info, stack)
}
