# TLS Eino Demo Example

This module is itself a runnable Eino application. Configure the TLS
environment variables described in the parent README, then run it from the
module root:

```bash
go run .
```

The command executes both non-streaming and streaming Eino chains and exports
their callback spans to the configured TLS trace topic.
