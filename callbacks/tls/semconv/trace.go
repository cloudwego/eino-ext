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

package semconv

const (
	EinoImportPath = "github.com/cloudwego/eino"
)

const (
	GEN_AI_USAGE_COMPLETION_TOKENS     = "gen_ai.usage.completion_tokens"
	GEN_AI_USAGE_PROMPT_TOKENS         = "gen_ai.usage.prompt_tokens"
	GEN_AI_USAGE_INPUT_TOKENS          = "gen_ai.usage.input_tokens"
	GEN_AI_USAGE_OUTPUT_TOKENS         = "gen_ai.usage.output_tokens"
	GEN_AI_REQUEST_MAX_TOKENS          = "gen_ai.request.max_tokens"
	GEN_AI_REQUEST_TOP_P               = "gen_ai.request.top_p"
	GEN_AI_REQUEST_TEMPERATURE         = "gen_ai.request.temperature"
	GEN_AI_REQUEST_STOP                = "gen_ai.request.stop"
	GEN_AI_RESPONSE_MODEL              = "gen_ai.response.model"
	GEN_AI_REQUEST_ID                  = "gen_ai.request.id"
	GEN_AI_REQUEST_N                   = "gen_ai.request.n"
	GEN_AI_REQUEST_DURATION_MS         = "gen_ai.request.duration_ms"
	GEN_AI_USAGE_NUM_SEQUENCES         = "gen_ai.usage.num_sequences"
	GEN_AI_LATENCY_TIME_IN_QUEUE       = "gen_ai.latency.time_in_queue"
	GEN_AI_LATENCY_TIME_TO_FIRST_TOKEN = "gen_ai.latency.time_to_first_token"
	GEN_AI_LATENCY_E2E                 = "gen_ai.latency.e2e"
)

const (
	GEN_AI_TOKEN_TYPE       = "gen_ai.token.type"
	GEN_AI_INPUT            = "input.value"
	GEN_AI_OUTPUT           = "output.value"
	GEN_AI_INPUT_MESSAGES   = "gen_ai.input.messages"
	GEN_AI_OUTPUT_MESSAGES  = "gen_ai.output.messages"
	OUTPUT_MIME_TYPE        = "output.mime_type"
	INPUT_VALUE             = "input.value"
	INPUT_MIME_TYPE         = "input.mime_type"
	EMBEDDING_EMBEDDINGS    = "embedding.embeddings"
	GEN_AI_PROMPT           = "gen_ai.prompt"
	GEN_AI_COMPLETION       = "gen_ai.completion"
	GEN_AI_PROMPT_TEMPLATE  = "gen_ai.prompt_template.template"
	GEN_AI_PROMPT_VARIABLES = "gen_ai.prompt_template.variables"
	GEN_AI_PROMPT_VERSION   = "gen_ai.prompt_template.version"

	GEN_AI_SYSTEM = "gen_ai.system"
	TASK_NAME     = "task.name"
	TLS_APP_TYPE  = "tls.app.type"

	GEN_AI_OPERATION_NAME = "gen_ai.operation.name"
	GEN_AI_PROVIDER_NAME  = "gen_ai.provider.name"

	GEN_AI_SPAN_KIND = "gen_ai.span.kind"

	GEN_AI_SPAN_SUB_KIND = "gen_ai.span.sub_kind"

	SESSION_ID        = "session.id"
	GEN_AI_SESSION_ID = "gen_ai.session.id"

	USER_ID        = "user.id"
	GEN_AI_USER_ID = "gen_ai.user.id"

	GEN_AI_USER_NAME = "gen_ai.user.name"

	GEN_AI_FRAMEWORK = "gen_ai.framework"

	GEN_AI_USAGE_TOTAL_TOKENS = "gen_ai.usage.total_tokens"

	GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS     = "gen_ai.usage.cache_read_input_tokens"
	GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2  = "gen_ai.usage.cache_read.input_tokens"
	GEN_AI_USAGE_CACHED_TOKENS               = "gen_ai.usage.cached_tokens"
	GEN_AI_USAGE_CACHE_CREATE_INPUT_TOKENS   = "gen_ai.usage.cache_create_input_tokens"
	GEN_AI_USAGE_CACHE_CREATION_INPUT_TOKENS = "gen_ai.usage.cache_creation.input_tokens"
	GEN_AI_USAGE_REASONING_OUTPUT_TOKENS     = "gen_ai.usage.reasoning.output_tokens"

	GEN_AI_REQUEST_IS_STREAM = "gen_ai.request.is_stream"
	GEN_AI_IS_STREAMING      = "gen_ai.is_streaming"

	GEN_AI_REQUEST_MODEL = "gen_ai.request.model"

	GEN_AI_REQUEST_STOP_SEQUENCES = "gen_ai.request.stop_sequences"

	GEN_AI_REQUEST_TOOL_DEFINITIONS = "gen_ai.tool.definitions"

	GEN_AI_REQUEST_PARAMETERS = "gen_ai.request.parameters"

	GEN_AI_RESPONSE_FINISH_REASON = "gen_ai.response.finish_reason"

	GEN_AI_REASONING_CONTENT = "gen_ai.response.reasoning_content"

	GEN_AI_USER_TIME_TO_FIRST_TOKEN = "gen_ai.user.time_to_first_token"

	GEN_AI_RESPONSE_TIME_TO_FIRST_TOKEN = "gen_ai.response.time_to_first_token"

	GEN_AI_RESPONSE_TIME_OF_FIRST_TOKEN = "gen_ai.response.time_of_first_token"

	GEN_AI_MODEL_NAME = "gen_ai.model_name"

	VOLC_TRACE_FLAG = "volc.trace.flag"

	CONTENT = "content"
)

const (
	MESSAGE_ROLE                         = "message.role"
	MESSAGE_CONTENT                      = "message.content"
	MESSAGE_NAME                         = "message.name"
	MESSAGE_TOOL_CALLS                   = "message.tool_calls"
	MESSAGE_FUNCTION_CALL_NAME           = "message.function_call_name"
	MESSAGE_FUNCTION_CALL_ARGUMENTS_JSON = "message.function_call_arguments_json"
)

const (
	DOCUMENT_ID = "document.id"

	DOCUMENT_SCORE = "document.score"

	DOCUMENT_CONTENT = "document.content"

	DOCUMENT_METADATA = "document.metadata"
)

const (
	RERANKER_INPUT_DOCUMENTS  = "reranker.input_documents"
	RERANKER_OUTPUT_DOCUMENTS = "reranker.output_documents"
	RERANKER_QUERY            = "reranker.query"
	RERANKER_MODEL_NAME       = "reranker.model_name"
	RERANKER_TOP_K            = "reranker.top_k"
)

const (
	GEN_AI_RETRIEVAL_QUERY     = "gen_ai.retrieval.query"
	GEN_AI_RETRIEVAL_DOCUMENTS = "gen_ai.retrieval.documents"
)

const (
	EMBEDDING_TEXT                    = "embedding.text"
	EMBEDDING_VECTOR                  = "embedding.vector"
	EMBEDDING_VECTOR_SIZE             = "embedding.vector_size"
	GEN_AI_EMBEDDINGS_DIMENSION_COUNT = "gen_ai.embeddings.dimension.count"
)

const (
	GEN_AI_TOOL_CALL_ID            = "gen_ai.tool_call.id"
	GEN_AI_TOOL_DESCRIPTION        = "gen_ai.tool.description"
	GEN_AI_TOOL_NAME               = "gen_ai.tool.name"
	GEN_AI_TOOL_TYPE               = "gen_ai.tool.type"
	GEN_AI_TOOL_CALL_ARGUMENTS     = "gen_ai.tool.call.arguments"
	GEN_AI_TOOL_CALL_RESULT        = "gen_ai.tool.call.result"
	TOOL_SUCCESS                   = "tool.success"
	TOOL_ERROR                     = "tool.error"
	TOOL_CALL_FUNCTION_NAME        = "tool_call.function.name"
	TOOL_CALL_FUNCTION_ARGUMENTS   = "tool_call.function.arguments"
	TOOL_CALL_FUNCTION_DESCRIPTION = "tool_call.function.description"
	TOOL_CALL_FUNCTION_THOUGHTS    = "tool_call.function.thoughts"
)

const (
	AGENT_ACTION             = "agent.action"
	AGENT_ACTION_INPUT       = "agent.action_input"
	AGENT_INTERMEDIATE_STEPS = "agent.intermediate_steps"
	AGENT_PLAN               = "agent.plan"
)

const (
	MCP_METHOD_NAME           = "mcp.method.name"
	MCP_METHOD_ARGUMENTS_JSON = "mcp.method.arguments"
	MCP_METHOD_DESCRIPTION    = "mcp.method.description"
)

const (
	XTRACE      = "xtrace"
	TLS         = "tls"
	LLAMA_INDEX = "llama_index"

	BUFFER = "buffer"
	VECTOR = "vector"
	KVDB   = "kv-db"
	SQLDB  = "sql-db"

	CHAT       = "chat"
	COMPLETION = "completion"
	WORKFLOW   = "workflow"

	TEXT = "text/plain"
	JSON = "application/json"
)

const (
	GEN_AI_SPAN_KIND_LLM       = "LLM"
	GEN_AI_SPAN_KIND_TOOL      = "TOOL"
	GEN_AI_SPAN_KIND_CHAIN     = "CHAIN"
	GEN_AI_SPAN_KIND_RETRIEVER = "RETRIEVER"
	GEN_AI_SPAN_KIND_EMBEDDING = "EMBEDDING"
	GEN_AI_SPAN_KIND_AGENT     = "AGENT"
	GEN_AI_SPAN_KIND_RERANKER  = "RERANKER"
	GEN_AI_SPAN_KIND_TASK      = "TASK"
	GEN_AI_SPAN_KIND_DOCUMENT  = "DOCUMENT"
)
