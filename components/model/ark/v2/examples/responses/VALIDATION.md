# Responses code example validation

Latest rerun: [2026-11-04 Chat and Responses validation](../VALIDATION.md).
The results below preserve the 2026-11-02 run.

Date: 2026-11-02 (Asia/Singapore). Model: `doubao-seed-2-1-pro-260628`.

All 13 standalone programs compiled with Go 1.25.1. Each was run as its own
executable, without command-line options. Credentials and variable inputs came
from the environment. An external validation harness checked the output; the
examples themselves contain API usage and ordinary error handling.

## Live runs

14 executions: 10 passed content checks, 3 returned service permission errors,
and 1 completed without producing the correct PDF tool-result content.

| Case | Program directory | Process exit | External check |
| --- | --- | --- | --- |
| `generate` | `generate` | 0 | Passed |
| `stream` | `stream` | 0 | Passed |
| `tools` | `tools` | 0 | Passed |
| `structured` | `structured_output` | 0 | Passed |
| `reasoning` | `reasoning` | 0 | Passed |
| `modifiers` | `modifiers` | 0 | Passed |
| `image-base64` | `multimodal` | 0 | Passed |
| `pdf-base64` | `file` | 0 | Passed |
| `pdf-upload` | `upload` | 0 | Passed |
| `session` | `session_cache` | 1 | 403 AccessDenied.CacheService |
| `prefix` | `prefix_cache` | 1 | 403 AccessDenied.CacheService |
| `web-search` | `web_search` | 1 | 404 ToolNotOpen |
| `pdf-file-id` | `file` | 0 | Passed |
| `pdf-tool-result` | `tool_result` | 0 | Failed: PDF_OK_731 was not returned |

The harness supplied `ARK_PROMPT` to check EXAMPLE_OK, STREAM_OK, strict JSON
answer 21, and MODIFIER_OK. It checked the fixed streamed tool addition and
reasoning follow-up (324), original-response byte output and exactly one stream
completion marker, PDF input by base64 and FileID, and temporary-upload cleanup.
The image example used the bundled EINO logo. It produced a nonempty description;
this does not prove that every detail of the description was correct.

The `upload` program deleted its own temporary PDF. The harness also created
one PDF FileID for the `file` and `tool_result` programs and deleted it afterward.
Neither program deletes a caller-supplied `ARK_FILE_ID`.

The tool-result example exited normally, but the external marker check failed.
This distinguishes successful API execution from correct model interpretation.
Cache and search errors were retained; no service was activated for these tests.
Audio/video URL inputs are shown in code but were not live-tested in this run.

## Local HTTP/SSE checks

Six additional executions against a local server passed their expected checks:

- Session cache stored the first response and sent only incremental input with
  the previous response ID.
- Prefix cache created a stored prefix and used its ID for the follow-up.
- Web Search serialized the configured tool and beta header.
- A truncated SSE stream returned an unexpected EOF error.
- The tool-result program rejected a missing ARK_FILE_ID.
- Missing credentials were rejected before HTTP dispatch.

Local fixture responses establish request/control-flow behavior, not live account
access or model capabilities. The module race suite and `go vet ./...` passed;
all standalone example packages compiled. Formatting, spelling, and diff checks
also passed.
