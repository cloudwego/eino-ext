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

package metadata

import "context"

type metadataKey struct{}

type metadata map[string]string

// WithValue injects key/val pair into ctx.
// If there is duplicate key, original val would be overwritten
func WithValue(ctx context.Context, key, val string) context.Context {
	md, ok := ctx.Value(metadataKey{}).(metadata)
	if ok {
		md[key] = val
		return ctx
	}
	newMd := metadata(make(map[string]string))
	newMd[key] = val
	return context.WithValue(ctx, metadataKey{}, newMd)
}

// GetValue extracts related val with key.
// If key does not exist, would return "", false
func GetValue(ctx context.Context, key string) (string, bool) {
	md, ok := ctx.Value(metadataKey{}).(metadata)
	if ok {
		res, exist := md[key]
		return res, exist
	}
	return "", false
}

// GetAllValues extracts all key/val pairs.
// If there is no key/val pairs at all, would return nil, false
func GetAllValues(ctx context.Context) (map[string]string, bool) {
	md, ok := ctx.Value(metadataKey{}).(metadata)
	if ok {
		return md, true
	}
	return nil, false
}
