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

package qianfan

const (
	defaultTemperature       = float32(0.95)
	defaultTopP              = float32(0.7)
	defaultParallelToolCalls = true
)

const (
	toolChoiceNone     = "none"     // 不希望模型调用任何function，只生成面向用户的文本消息
	toolChoiceAuto     = "auto"     // 模型会根据输入内容自动决定是否调用函数以及调用哪些function
	toolChoiceRequired = "required" // 希望模型总是调用一个或多个function
)

func of[T any](v T) *T {
	return &v
}
