# Responses stream replay fixtures

Captured on 2026-11-09 using `doubao-seed-2-1-pro-260628` with the text,
reasoning and function-tool examples. The `.sse` files preserve the observed
event order and text/argument delta boundaries. The `.json` files record the
inspected result of `Stream` followed by `schema.ConcatMessages`.

Response, item, call and request IDs, creation timestamps and encrypted reasoning
values are replaced with deterministic fixtures. No API credentials or original
encrypted values are included. Usage and ordinary model text are retained.
Tests replay these files through a local HTTP server and make no external calls.
