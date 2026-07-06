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

// Package boxlite provides a filesystem.Backend (plus Shell and StreamingShell)
// backed by a BoxLite microVM, so an ADK agent's whole workspace — file reads,
// writes, edits, searches, and shell commands — runs inside hardware-isolated
// sandbox instead of on the host.
//
// Where the local backend executes on the host and agentkit delegates to a
// remote sandbox service, BoxLite boots a self-hosted microVM per backend:
// a dedicated guest kernel with no shared host state, suited to running
// untrusted, model-generated commands.
//
// The backend uses the BoxLite Go SDK, which is CGO with a prebuilt native
// library, so the implementation lives behind the "boxlite" build tag. Install
// the native library once, then build/run with the tag:
//
//	go run github.com/boxlite-ai/boxlite/sdks/go/cmd/setup
//	go build -tags boxlite ./...
//
// Without the tag this package is empty on purpose: it keeps `go build ./...`
// green on toolchains that don't have the native library (CI, unsupported
// platforms) while consumers opt in with -tags boxlite. Supported platforms are
// linux/amd64 and darwin/arm64; macOS additionally needs the Hypervisor
// entitlement to boot a box.
package boxlite
