# BoxLite Backend

A `filesystem.Backend` (plus `Shell` and `StreamingShell`) backed by a [BoxLite](https://github.com/boxlite-ai/boxlite) microVM — the ADK agent's whole workspace (file reads, writes, edits, searches, and shell commands) runs inside a hardware-isolated sandbox instead of on the host.

Where the `local` backend executes on the host and `agentkit` delegates to a remote sandbox service, BoxLite boots a **self-hosted microVM** per backend: a dedicated guest kernel, no shared host state, no external service. That's the boundary you want when agents run untrusted, model-generated commands.

## Install

The BoxLite SDK is CGO with a prebuilt native library, so installation is two steps:

```bash
go get github.com/boxlite-ai/boxlite/sdks/go
go run github.com/boxlite-ai/boxlite/sdks/go/cmd/setup   # downloads libboxlite.a + header
```

Requirements: `CGO_ENABLED=1`, one of `darwin/arm64` or `linux/amd64`, and a hypervisor to boot boxes (KVM on Linux; the Hypervisor entitlement on macOS).

Because of that native dependency, the implementation is behind the **`boxlite` build tag** — without it the package is empty, so pure-Go/CI builds (`go build ./...`) stay green:

```bash
go build -tags boxlite ./...
go test  -tags boxlite ./...
```

## Quick start

```go
backend, err := boxlite.NewBackend(ctx, &boxlite.Config{})
if err != nil {
    log.Fatal(err)
}
if err := backend.Create(ctx); err != nil { // boots the microVM
    log.Fatal(err)
}
defer backend.Cleanup(ctx)

// Use it anywhere ADK accepts a filesystem.Backend / Shell / StreamingShell.
_ = backend.Write(ctx, &filesystem.WriteRequest{FilePath: "hello.py", Content: "print(6*7)"})
out, _ := backend.Execute(ctx, &filesystem.ExecuteRequest{Command: "python3 hello.py"})
log.Println(out.Output) // 42
```

Configure the box via `Config` (image, CPUs, memory, network, per-file-op timeout, `ValidateCommand`) and the runtime via `Config.RuntimeOptions` (image registry, data dir).

## How it maps

Every operation goes through in-guest exec so reads, writes, and commands share one filesystem view (BoxLite's `copy_in`/`copy_out` observe a different layer than in-guest exec, so they are deliberately not used). Guest scripts take paths as positional parameters — no shell-quoting of model input.

| Interface method | In-guest realization |
| --- | --- |
| `LsInfo` | `ls -1A` |
| `Read` | `sed -n 'start,endp'` (line window; `Limit<=0` caps at 2000 lines like `local`) |
| `GrepRaw` | `rg --json` (requires ripgrep in the guest image; friendly error otherwise) |
| `GlobInfo` | `find -mindepth 1` + host-side `doublestar` matching |
| `Write` | base64 chunks under the kernel per-arg limit → unbounded file size |
| `Edit` | base64 read → `local`-parity occurrence semantics host-side → chunked write |
| `Execute` | `sh -c` via `Box.Exec` (non-zero exit → `local`'s combined report format) |
| `ExecuteStreaming` | `Box.StartExecution` + `OnStdout` line streaming + drain-barrier `Wait` |

Not implemented (yet): the optional `MultiModalReader` interface (image/PDF rendering) — planned as a follow-up.

## Choosing a backend

| | BoxLite | local | agentkit |
| --- | --- | --- | --- |
| Isolation | microVM, own kernel | none (host) | remote sandbox service |
| Deployment | self-hosted | in-process | cloud service |
| Platforms | `linux/amd64`, `darwin/arm64` | Unix/macOS | anywhere (client) |
| Dependency | CGO + native lib | pure Go | network + credentials |

Use BoxLite when you want hardware isolation without an external service; `local` for trusted code; `agentkit` for managed cloud sandboxes.
