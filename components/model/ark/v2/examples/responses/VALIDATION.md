# Responses example validation

Date: 2026-11-02 (Asia/Singapore). Model: `doubao-seed-2-1-pro-260628`.

The example was built with Go 1.25.1 and run as a standalone executable for each
case below. All inference calls went through `NewResponsesAPIChatModel` from the
example; this record is distinct from the adapter's opt-in Go integration tests.
Credentials were provided only through the process environment.

```sh
GOTOOLCHAIN=go1.25.1 GOWORK=off go build -o /tmp/eino-responses-example ./examples/responses
# Equivalent entry point for the documented commands:
go run ./examples/responses -mode tools -expect 21
```

## Live runs

14 commands: 10 succeeded, 3 returned service permission errors, and 1 failed
content verification. Failures retain nonzero exit status.

| Case | Exit | Seconds | Result |
| --- | --- | --- | --- |
| `generate` | 0 | 0.99 | Passed |
| `stream` | 0 | 0.72 | Passed |
| `tools` | 0 | 8.58 | Passed |
| `structured` | 0 | 1.28 | Passed |
| `reasoning` | 0 | 7.38 | Passed |
| `modifiers` | 0 | 2.29 | Passed |
| `image-base64` | 0 | 8.06 | Passed |
| `image-upload` | 0 | 5.56 | Passed |
| `pdf-base64` | 0 | 2.84 | Passed |
| `pdf-upload` | 0 | 1.98 | Passed |
| `session` | 1 | 0.22 | 403 AccessDenied.CacheService; cache service not activated |
| `prefix` | 1 | 0.16 | 403 AccessDenied.CacheService; cache service not activated |
| `web-search` | 1 | 2.20 | 404 ToolNotOpen; search service not activated |
| `pdf-tool-result` | 1 | 6.79 | Content check failed; the model did not return PDF_OK_731 |

The `tools` example assembled `{"a":8,"b":13}`, executed addition locally, and
returned 21 to the model. Strict structured output decoded as `{"answer":"21"}`.
Reasoning was streamed and passed back, and the follow-up returned 324.
The modifier example observed two request modifications, one original response,
11 original SSE events, and exactly one successful finalization. Image descriptions
matched the supplied EINO logo's lettering and rocket design; individual wording
was not checked for exact equality. Both PDF input paths returned `PDF_OK_731`.

`upload` paths created and successfully deleted three temporary provider files:
image input, PDF input, and PDF tool result. The last file was also deleted when
content verification failed. Existing FileIDs were not deleted.

The PDF tool-result request completed, but the model replied that the media was
not available to access. `-expect PDF_OK_731` caught the incorrect answer. This
run establishes a content failure for this model; it does not establish a general
absence of multimodal tool-result support.

The tiny fixture PDF is synthetic and contains no user data. SHA256:
`573305c3eaca303ba9e50db4bb1ffaaf715fb1b3a23791dfed3082ef30c772ba`.

## Local HTTP checks

Eight additional runs against a local HTTP/SSE server passed their expected
outcomes:

| Case | Expected outcome |
| --- | --- |
| Session cache | Store first response, reuse its ID, send only the incremental message, verify CACHE_731 |
| Prefix cache | Create a stored prefix, pass its ID to the next request, verify PREFIX_731 |
| Web Search | Serialize web_search with limit=2 and ark-beta-web-search=true |
| Wrong expected text | Nonzero exit when the final answer lacks the marker |
| Truncated stream | Nonzero exit with unexpected EOF |
| Media with tools mode | Reject flags that would otherwise be ignored |
| Missing media | Reject before inference |
| Invalid thinking mode | Reject before inference |

Local responses are fixtures; they do not validate live account access, cache
hits, search results, or model media understanding.
