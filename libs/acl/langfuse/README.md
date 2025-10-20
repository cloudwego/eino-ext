# Langfuse ACL Extension

This module provides the Langfuse adapter used by Eino for event and prompt management against the Langfuse public API.

- `prompt.go` implements the prompt client, including typed request/response models and helpers to list, fetch, and create prompts.
- A minimal usage sample lives in `examples/prompt/main.go`, showing how to initialize a Langfuse client and load a prompt by name.

Run `go test ./...` before submitting changes to ensure the package continues to build and the existing suites pass.
