# Responses stream replay fixtures

Captured on 2026-11-09 using `doubao-seed-2-1-pro-260628` with the text,
reasoning and function-tool examples. The `.sse` files preserve the observed
event order and text/argument delta boundaries. The `.json` files record the
inspected result of `Stream` followed by `schema.ConcatMessages`.

Response, item, call and request IDs, creation timestamps and encrypted reasoning
values are replaced with deterministic fixtures. No API credentials or original
encrypted values are included. Usage and ordinary model text are retained.
Tests replay these files through a local HTTP server and make no external calls.

`mixed.sse` is a hand-authored protocol fixture, not a captured model response.
It interleaves two function calls with two reasoning summaries, raw reasoning,
and three visible text parts across two message items. Its encrypted signature
arrives only in the terminal response. Native output indices are 0 through 4;
function calls occupy output indices 2 and 3 but allocate no part indices.
Their independent ToolCall.Index values are 0 and 1; native output indices remain
in Extra for history ordering.
`TestResponsesMixedStreamConcat` checks the six part indices, the two tool indices,
completed/incomplete responses, both Concat APIs, and JSON/Sonic/gob history.
