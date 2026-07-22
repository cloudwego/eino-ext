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

package sqlitevec

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func normalizeTableName(name, fallback string) (string, error) {
	if name == "" {
		name = fallback
	}
	if !identifierPattern.MatchString(name) {
		return "", fmt.Errorf("invalid table name %q", name)
	}
	return name, nil
}

func vectorToJSON(vector []float64, vectorDim int) (string, error) {
	if len(vector) != vectorDim {
		return "", fmt.Errorf("invalid vector dimension, expected=%d, got=%d", vectorDim, len(vector))
	}
	b, err := json.Marshal(vector)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func metadataFromJSON(metadataJSON string) (map[string]any, error) {
	if metadataJSON == "" {
		return map[string]any{}, nil
	}
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		return map[string]any{}, nil
	}
	return metadata, nil
}
