/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticark

import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[blockExtraItemID]("_eino_ext_ark_content_block_extra_item_id")
	schema.RegisterName[blockExtraItemStatus]("_eino_ext_ark_content_block_extra_item_status")
	schema.RegisterName[*ResponseMetaExtension]("_eino_ext_ark_response_meta_extension")
	schema.RegisterName[*AssistantGenTextExtension]("_eino_ext_ark_assistant_gen_text_extension")
	schema.RegisterName[*ServerToolCallArguments]("_eino_ext_ark_server_tool_call_arguments")

	compose.RegisterStreamChunkConcatFunc(concatFirstNonZero[blockExtraItemID])
	compose.RegisterStreamChunkConcatFunc(concatLast[blockExtraItemStatus])
	compose.RegisterStreamChunkConcatFunc(concatResponseMetaExtensions)
	compose.RegisterStreamChunkConcatFunc(concatAssistantGenTextExtensions)
	compose.RegisterStreamChunkConcatFunc(concatServerToolCallArguments)
}
