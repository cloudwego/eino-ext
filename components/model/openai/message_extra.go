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

package openai

import (
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

// GetResponseID returns the Chat Completions response body's id, not the HTTP
// X-Request-ID header. It works with Generate results, Stream chunks, and messages
// combined with schema.ConcatMessages. It returns an empty string if absent.
func GetResponseID(msg *schema.Message) string {
	return openai.GetRequestID(msg)
}
