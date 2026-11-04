# Live example validation

Date: 2026-11-04, completed at 16:34 (Asia/Singapore).
Model: `doubao-seed-2-1-pro-260628`. Tested adapter revision: `2b0f62459473dfa27ca375edd7e580beb327a5be`.

All 20 standalone examples (7 Chat Completions and 13 Responses) were rebuilt
with Go 1.25.1, `GOWORK=off`, and the committed module dependencies. The external
driver ran the compiled programs against the real Ark service with the authorized
API Key supplied through process environment variables. Credentials and account
identifiers are omitted from this report. Responses examples used environment
configuration; the existing Chat media example used its documented arguments.

## Results

23 live executions: 19 passed the external checks, 3 returned service permission
errors, and 1 returned successfully without reading the PDF tool-result marker.

- Chat Completions: 9/9 passed.
- Responses: 10/14 passed; 3 service permission errors; 1 content-check failure.

| Case | Example directory | Exit | External result |
| --- | --- | --- | --- |
| `chat-generate` | `generate` | 0 | Passed |
| `chat-image-base64` | `multimodal` | 0 | Passed |
| `chat-modifiers` | `modifiers` | 0 | Passed |
| `chat-pdf-base64` | `multimodal` | 0 | Passed |
| `chat-pdf-file-id` | `multimodal` | 0 | Passed |
| `chat-pdf-tool-result` | `tool_result` | 0 | Passed |
| `chat-stream` | `stream` | 0 | Passed |
| `chat-structured` | `structured_output` | 0 | Passed |
| `chat-tools` | `tools` | 0 | Passed |
| `responses-generate` | `responses/generate` | 0 | Passed |
| `responses-image-base64` | `responses/multimodal` | 0 | Passed |
| `responses-modifiers` | `responses/modifiers` | 0 | Passed |
| `responses-pdf-base64` | `responses/file` | 0 | Passed |
| `responses-pdf-file-id` | `responses/file` | 0 | Passed |
| `responses-pdf-tool-result` | `responses/tool_result` | 0 | Failed: PDF marker not returned |
| `responses-pdf-upload` | `responses/upload` | 0 | Passed |
| `responses-prefix-cache` | `responses/prefix_cache` | 1 | 403 `AccessDenied.CacheService` |
| `responses-reasoning` | `responses/reasoning` | 0 | Passed |
| `responses-session-cache` | `responses/session_cache` | 1 | 403 `AccessDenied.CacheService` |
| `responses-stream` | `responses/stream` | 0 | Passed |
| `responses-structured` | `responses/structured_output` | 0 | Passed |
| `responses-tools` | `responses/tools` | 0 | Passed |
| `responses-web-search` | `responses/web_search` | 1 | 404 `ToolNotOpen` |

## Checks and limitations

Text/stream checks verified nonempty answers and final usage. Structured examples
checked Singapore/3 days and strict JSON answer 21. Tool examples assembled
streamed arguments, executed `add(8, 13)`, and returned its result to the model.
The reasoning follow-up returned 324. Modifier checks verified positive raw
response lengths and exactly one successful stream-completion marker.

Image checks required a nonempty description of the bundled EINO logo; they do
not verify every visual detail. PDF checks required the exact marker
`PDF_OK_731` from the bundled synthetic document. Both APIs read it from base64
and FileID input. Chat also read it from the document tool result.

The Responses PDF tool-result request exited 0, but its answer said the document
content had not loaded and did not contain the marker. The same processed FileID
worked as direct input. This reproduces the previously recorded limitation; it
does not establish a regression or identify its cause.

Session/prefix cache still returned 403 because this account has not activated
cache service for the model. Built-in search still returned 404 because web
search has not been activated. No service was activated for this run. These
outcomes match the [2026-11-02 Responses run](responses/VALIDATION.md).

The external Chat stream checker initially used a single-line reasoning regex.
The captured reasoning began with a newline; parsing the complete multiline
field confirmed nonempty reasoning and a successful final answer/usage. The
checker was corrected and the captured output revalidated without another API
call; the initial result was retained with the local run artifacts.

## Resource cleanup and scope

The upload example deleted its own temporary PDF. The external driver created
one separate processed PDF for the FileID and tool-result cases and deleted it
after those executions. Retrieving each of the two files afterward returned 404,
confirming cleanup. No pre-existing files were deleted.

Raw outputs and external checks remain in local run artifacts outside the
repository. This rerun used real inference, not HTTP mocks. Audio/video variants
and production Worker/History persistence were not tested in this run; JSON/gob
restoration and mocked history continuation are covered by separate adapter tests.
