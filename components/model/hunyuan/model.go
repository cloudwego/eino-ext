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

package hunyuan

// read https://cloud.tencent.com/document/product/1729/104753 for more supported models.
const (
	Lite            string = "hunyuan-lite"
	Standard        string = "hunyuan-standard"
	Standard256k    string = "hunyuan-standard-256K"
	Large           string = "hunyuan-large"
	Turbo           string = "hunyuan-turbo"
	TurboLatest     string = "hunyuan-turbo-latest"
	T1              string = "hunyuan-t1-latest"
	Translation     string = "hunyuan-translation"
	TranslationLite string = "hunyuan-translation-lite"
	Role            string = "hunyuan-role"
	FunctionCall    string = "hunyuan-functioncall"
	Code            string = "hunyuan-code"
)
