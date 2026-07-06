//go:build boxlite

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

// Implementation notes (the local backend is the behavioral reference):
//
//   - Every operation goes through in-guest exec. BoxLite's copy_in/copy_out
//     observe a different filesystem layer than in-guest exec, so a copied-in
//     file would be invisible to a later Execute; routing file I/O through the
//     shell keeps reads, writes, and commands on one consistent view.
//   - Guest scripts receive paths as positional parameters ("$1", "$2", ...)
//     rather than interpolated strings, so no shell-quoting of model-supplied
//     paths is ever needed.
//   - Write streams content in base64 chunks sized under the kernel's per-arg
//     limit (MAX_ARG_STRLEN, 128 KiB on Linux), so file size is not bounded by
//     a single argv.
package boxlite

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	sdk "github.com/boxlite-ai/boxlite/sdks/go"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultImage         = "python:3.9-slim"
	defaultWorkDir       = "/workspace"
	defaultCPUs          = 1
	defaultMemoryMiB     = 512
	defaultFileOpTimeout = 30 * time.Second
	defaultRootPath      = "/"

	// defaultReadLimit mirrors the local backend, which caps a Read with
	// Limit<=0 at 2000 lines (rather than the whole file) to keep tool output
	// bounded for the model.
	defaultReadLimit = 2000

	// writeChunkRawBytes is the raw payload per Write exec. Its base64 form
	// (~4/3x) must stay under Linux's 128 KiB per-argument limit.
	writeChunkRawBytes = 72 * 1024
)

// Config configures the BoxLite sandbox backend. Zero-value fields fall back
// to the defaults above.
type Config struct {
	Image          string        // OCI image; default "python:3.9-slim"
	Name           string        // optional human-readable box name
	WorkDir        string        // working dir inside the box; default "/workspace"
	CPUs           int           // virtual CPUs; default 1
	MemoryMiB      int           // memory limit in MiB; default 512
	NetworkEnabled bool          // default false (fully isolated)
	FileOpTimeout  time.Duration // timeout for internal file operations; default 30s.
	// Execute/ExecuteStreaming deliberately have no default timeout (they honor
	// ctx only), matching the local backend.

	// ValidateCommand vets a command before Execute/ExecuteStreaming runs it.
	// Optional; nil allows everything (the microVM is the security boundary).
	ValidateCommand func(string) error

	// RuntimeOptions is passed through to sdk.NewRuntime — use it to point at an
	// image registry or a custom data dir. Optional.
	RuntimeOptions []sdk.RuntimeOption
}

// Sandbox is a filesystem.Backend, filesystem.Shell, and
// filesystem.StreamingShell backed by a single BoxLite microVM.
// Create boots the VM; Cleanup tears it down.
type Sandbox struct {
	config          Config
	validateCommand func(string) error
	runtime         *sdk.Runtime
	box             *sdk.Box
}

var (
	_ filesystem.Backend        = (*Sandbox)(nil)
	_ filesystem.Shell          = (*Sandbox)(nil)
	_ filesystem.StreamingShell = (*Sandbox)(nil)
)

// NewBackend builds a sandbox backend from config and applies defaults. It does
// NOT boot the microVM — call Create for that.
func NewBackend(_ context.Context, cfg *Config) (*Sandbox, error) {
	c := Config{}
	if cfg != nil {
		c = *cfg
	}
	if c.Image == "" {
		c.Image = defaultImage
	}
	if c.WorkDir == "" {
		c.WorkDir = defaultWorkDir
	}
	if c.CPUs == 0 {
		c.CPUs = defaultCPUs
	}
	if c.MemoryMiB == 0 {
		c.MemoryMiB = defaultMemoryMiB
	}
	if c.FileOpTimeout == 0 {
		c.FileOpTimeout = defaultFileOpTimeout
	}
	validate := c.ValidateCommand
	if validate == nil {
		validate = func(string) error { return nil }
	}
	return &Sandbox{config: c, validateCommand: validate}, nil
}

// Create boots the microVM: new runtime -> create box -> start -> ensure workdir.
// On any failure it unwinds whatever it already allocated so nothing leaks.
func (s *Sandbox) Create(ctx context.Context) error {
	rt, err := sdk.NewRuntime(s.config.RuntimeOptions...)
	if err != nil {
		return fmt.Errorf("boxlite: create runtime: %w", err)
	}

	network := sdk.NetworkSpec{Mode: sdk.NetworkModeDisabled}
	if s.config.NetworkEnabled {
		network.Mode = sdk.NetworkModeEnabled
	}

	opts := []sdk.BoxOption{
		sdk.WithCPUs(s.config.CPUs),
		sdk.WithMemory(s.config.MemoryMiB),
		sdk.WithWorkDir(s.config.WorkDir),
		sdk.WithNetwork(network),
		// Keep the box on Stop so Cleanup's ForceRemove is the single,
		// deterministic deletion point (otherwise auto-remove races Stop).
		sdk.WithAutoRemove(false),
	}
	if s.config.Name != "" {
		opts = append(opts, sdk.WithName(s.config.Name))
	}

	box, err := rt.Create(ctx, s.config.Image, opts...)
	if err != nil {
		_ = rt.Close()
		return fmt.Errorf("boxlite: create box: %w", err)
	}

	if err := box.Start(ctx); err != nil {
		teardown(ctx, rt, box)
		return fmt.Errorf("boxlite: start box: %w", err)
	}

	// The configured workdir may not exist in the image (nothing bind-mounts it
	// into a microVM). Bootstrap it from "/" so the exec itself has a valid cwd;
	// every later exec then inherits WorkDir as its cwd.
	mkdir := box.Command("mkdir", "-p", s.config.WorkDir)
	mkdir.Dir = "/"
	if err := mkdir.Run(ctx); err != nil {
		teardown(ctx, rt, box)
		return fmt.Errorf("boxlite: ensure workdir %q: %w", s.config.WorkDir, err)
	}

	s.runtime = rt
	s.box = box
	return nil
}

// Cleanup stops the box, removes it, and closes the runtime. Idempotent.
func (s *Sandbox) Cleanup(ctx context.Context) {
	var problems []string
	if s.box != nil {
		if err := s.box.Stop(ctx); err != nil {
			problems = append(problems, fmt.Sprintf("stop box: %v", err))
		}
		if s.runtime != nil {
			if err := s.runtime.ForceRemove(ctx, s.box.ID()); err != nil {
				problems = append(problems, fmt.Sprintf("remove box: %v", err))
			}
		}
		if err := s.box.Close(); err != nil {
			problems = append(problems, fmt.Sprintf("close box: %v", err))
		}
		s.box = nil
	}
	if s.runtime != nil {
		if err := s.runtime.Close(); err != nil {
			problems = append(problems, fmt.Sprintf("close runtime: %v", err))
		}
		s.runtime = nil
	}
	if len(problems) > 0 {
		log.Printf("[WARN] boxlite: cleanup: %s", strings.Join(problems, ", "))
	}
}

// runGuest executes an argv in the guest with the file-op timeout applied.
// A non-zero exit code is a normal result, not a Go error.
func (s *Sandbox) runGuest(ctx context.Context, argv ...string) (*sdk.ExecResult, error) {
	if s.box == nil {
		return nil, errors.New("boxlite: sandbox not initialized (call Create first)")
	}
	if s.config.FileOpTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.FileOpTimeout)
		defer cancel()
	}
	res, err := s.box.Exec(ctx, argv[0], argv[1:]...)
	if err != nil {
		return nil, fmt.Errorf("boxlite: exec %q: %w", argv[0], err)
	}
	return res, nil
}

// runGuestScript runs a shell script with positional parameters: the script
// sees args as "$1", "$2", ... so model-supplied paths never need quoting.
func (s *Sandbox) runGuestScript(ctx context.Context, script string, args ...string) (*sdk.ExecResult, error) {
	argv := append([]string{"sh", "-c", script, "sh"}, args...)
	return s.runGuest(ctx, argv...)
}

// resolvePath resolves a request path against the workdir. Absolute paths are
// used as-is — the microVM itself is the isolation boundary, so unlike a
// host-rooted backend there is no traversal surface to guard.
func (s *Sandbox) resolvePath(p string) string {
	if p == "" {
		return s.config.WorkDir
	}
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	return path.Join(s.config.WorkDir, p)
}

// ---- filesystem.Backend ----

const lsScript = `[ -e "$1" ] || exit 3
[ -d "$1" ] || exit 4
ls -1A "$1"`

func (s *Sandbox) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	p := s.resolvePath(req.Path)
	res, err := s.runGuestScript(ctx, lsScript, p)
	if err != nil {
		return nil, err
	}
	switch res.ExitCode {
	case 0:
	case 3:
		// Mirror the local backend: a missing directory lists as empty.
		return nil, nil
	case 4:
		return nil, fmt.Errorf("failed to read directory: not a directory: %s", p)
	default:
		return nil, fmt.Errorf("failed to read directory: %s", strings.TrimSpace(res.Stderr))
	}

	var files []filesystem.FileInfo
	for _, name := range splitLines(res.Stdout) {
		files = append(files, filesystem.FileInfo{Path: name})
	}
	return files, nil
}

const readScript = `[ -e "$1" ] || exit 3
[ -d "$1" ] && exit 4
sed -n "${2},${3}p" "$1"`

func (s *Sandbox) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	p := s.resolvePath(req.FilePath)

	offset := req.Offset
	if offset <= 0 {
		offset = 1
	}
	// Limit<=0 caps at 2000 lines, mirroring the local backend.
	limit := req.Limit
	if limit <= 0 {
		limit = defaultReadLimit
	}
	end := offset + limit - 1

	res, err := s.runGuestScript(ctx, readScript, p, fmt.Sprintf("%d", offset), fmt.Sprintf("%d", end))
	if err != nil {
		return nil, err
	}
	switch res.ExitCode {
	case 0:
	case 3:
		return nil, fmt.Errorf("file not found: %s", p)
	case 4:
		return nil, fmt.Errorf("failed to read file: path is a directory: %s", p)
	default:
		return nil, fmt.Errorf("failed to read file: %s", strings.TrimSpace(res.Stderr))
	}

	return &filesystem.FileContent{
		Content: strings.TrimSuffix(res.Stdout, "\n"),
	}, nil
}

// rgJSON mirrors ripgrep's --json "match"/"context" event shape (same struct
// the local backend parses).
type rgJSON struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		LineNumber int `json:"line_number"`
		Lines      struct {
			Text string `json:"text"`
		} `json:"lines"`
	} `json:"data"`
}

const rgProbeScript = `command -v rg >/dev/null 2>&1`

// GrepRaw searches inside the guest with ripgrep, matching the local backend's
// flag construction and JSON parsing. The guest image must contain `rg`; a
// missing binary returns the same install-hint error the local backend uses.
func (s *Sandbox) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	p := s.resolvePath(req.Path)

	// The guest agent reports a missing executable as a spawn error rather than
	// exit 127, so probe with POSIX `command -v` first — an exit-code answer,
	// no parsing of spawn-error prose.
	probe, err := s.runGuestScript(ctx, rgProbeScript)
	if err != nil {
		return nil, err
	}
	if probe.ExitCode != 0 {
		return nil, fmt.Errorf("ripgrep (rg) is not installed in the sandbox image. Please use an image with ripgrep, or install it: https://github.com/BurntSushi/ripgrep#installation")
	}

	argv := []string{"rg", "--json"}
	if req.CaseInsensitive {
		argv = append(argv, "-i")
	}
	if req.EnableMultiline {
		argv = append(argv, "-U", "--multiline-dotall")
	}
	if req.FileType != "" {
		argv = append(argv, "--type", req.FileType)
	} else if req.Glob != "" {
		argv = append(argv, "--glob", req.Glob)
	}
	if req.AfterLines > 0 {
		argv = append(argv, "-A", fmt.Sprintf("%d", req.AfterLines))
	}
	if req.BeforeLines > 0 {
		argv = append(argv, "-B", fmt.Sprintf("%d", req.BeforeLines))
	}
	argv = append(argv, "-e", req.Pattern, "--", p)

	res, err := s.runGuest(ctx, argv...)
	if err != nil {
		return nil, err
	}
	switch res.ExitCode {
	case 0:
	case 1: // ripgrep: no matches
		return []filesystem.GrepMatch{}, nil
	default:
		return nil, fmt.Errorf("ripgrep failed with exit code %d: %s", res.ExitCode, res.Stderr)
	}

	var matches []filesystem.GrepMatch
	for _, line := range splitLines(res.Stdout) {
		var data rgJSON
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data.Type != "match" && data.Type != "context" {
			continue
		}
		matchPath := data.Data.Path.Text
		if req.FileType != "" && req.Glob != "" {
			matched, _ := doublestar.Match(req.Glob, matchPath)
			if !matched {
				matched, _ = doublestar.Match(req.Glob, path.Base(matchPath))
			}
			if !matched {
				continue
			}
		}
		matches = append(matches, filesystem.GrepMatch{
			Path:    matchPath,
			Line:    data.Data.LineNumber,
			Content: strings.TrimRight(data.Data.Lines.Text, "\n"),
		})
	}
	return matches, nil
}

func (s *Sandbox) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	base := req.Path
	if base == "" {
		base = defaultRootPath
	}
	base = s.resolvePath(base)

	res, err := s.runGuest(ctx, "find", base, "-mindepth", "1")
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("failed to walk directory: %s", strings.TrimSpace(res.Stderr))
	}

	prefix := strings.TrimSuffix(base, "/") + "/"
	var matches []string
	for _, line := range splitLines(res.Stdout) {
		rel := strings.TrimPrefix(line, prefix)
		if rel == line && line != base {
			// Defensive: find should always emit under base.
			continue
		}
		matched, _ := doublestar.Match(req.Pattern, rel)
		if matched {
			matches = append(matches, rel)
		}
	}
	sort.Strings(matches)

	var files []filesystem.FileInfo
	for _, m := range matches {
		files = append(files, filesystem.FileInfo{Path: m})
	}
	return files, nil
}

const writeFirstScript = `mkdir -p "$(dirname "$1")" || exit 5
printf %s "$2" | base64 -d > "$1"`

const writeAppendScript = `printf %s "$2" | base64 -d >> "$1"`

const writeEmptyScript = `mkdir -p "$(dirname "$1")" || exit 5
: > "$1"`

// Write creates or replaces a file. Content travels as base64 chunks sized
// under the kernel per-arg limit, so file size is not bounded by one argv.
func (s *Sandbox) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	p := s.resolvePath(req.FilePath)

	if req.Content == "" {
		res, err := s.runGuestScript(ctx, writeEmptyScript, p)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("failed to write to file: %s", strings.TrimSpace(res.Stderr))
		}
		return nil
	}

	content := []byte(req.Content)
	for i := 0; i < len(content); i += writeChunkRawBytes {
		chunk := content[i:min(i+writeChunkRawBytes, len(content))]
		enc := base64.StdEncoding.EncodeToString(chunk)
		script := writeAppendScript
		if i == 0 {
			script = writeFirstScript
		}
		res, err := s.runGuestScript(ctx, script, p, enc)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("failed to write to file: %s", strings.TrimSpace(res.Stderr))
		}
	}
	return nil
}

const readB64Script = `[ -e "$1" ] || exit 3
base64 "$1"`

// Edit fetches the file (base64, binary-safe), applies the local backend's
// exact occurrence-count semantics host-side, and writes the result back.
func (s *Sandbox) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if req.OldString == "" {
		return fmt.Errorf("old string is required")
	}
	if req.OldString == req.NewString {
		return fmt.Errorf("new string must be different from old string")
	}
	p := s.resolvePath(req.FilePath)

	res, err := s.runGuestScript(ctx, readB64Script, p)
	if err != nil {
		return err
	}
	if res.ExitCode == 3 {
		return fmt.Errorf("failed to read file: file not found: %s", p)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("failed to read file: %s", strings.TrimSpace(res.Stderr))
	}
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(res.Stdout, "\n", ""))
	if err != nil {
		return fmt.Errorf("failed to decode file content: %w", err)
	}

	text := string(raw)
	count := strings.Count(text, req.OldString)
	if count == 0 {
		return fmt.Errorf("string not found in file: '%s'", req.OldString)
	}
	if count > 1 && !req.ReplaceAll {
		return fmt.Errorf("string '%s' appears multiple times. Use replace_all=True to replace all occurrences", req.OldString)
	}

	var newText string
	if req.ReplaceAll {
		newText = strings.ReplaceAll(text, req.OldString, req.NewString)
	} else {
		newText = strings.Replace(text, req.OldString, req.NewString, 1)
	}
	return s.Write(ctx, &filesystem.WriteRequest{FilePath: req.FilePath, Content: newText})
}

// ---- filesystem.Shell ----

// Execute runs a shell command to completion. Non-zero exits are reported in
// ExecuteResponse (with the local backend's combined output format), not as Go
// errors. No default timeout — the caller's ctx bounds it, like the local
// backend.
func (s *Sandbox) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (*filesystem.ExecuteResponse, error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}
	if s.box == nil {
		return nil, errors.New("boxlite: sandbox not initialized (call Create first)")
	}

	res, err := s.box.Exec(ctx, "sh", "-c", input.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	exitCode := res.ExitCode
	if exitCode != 0 {
		parts := []string{fmt.Sprintf("command exited with non-zero code %d", exitCode)}
		if res.Stdout != "" {
			parts = append(parts, "[stdout]:\n"+res.Stdout)
		}
		if res.Stderr != "" {
			parts = append(parts, "[stderr]:\n"+res.Stderr)
		}
		return &filesystem.ExecuteResponse{
			Output:   strings.Join(parts, "\n"),
			ExitCode: &exitCode,
		}, nil
	}
	return &filesystem.ExecuteResponse{
		Output:   res.Stdout,
		ExitCode: &exitCode,
	}, nil
}

// ---- filesystem.StreamingShell ----

// ExecuteStreaming streams stdout line-by-line while the command runs, then a
// completion message mirroring the local backend (non-zero exit -> combined
// stderr report; zero exit with no output -> empty response with code 0).
func (s *Sandbox) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}
	if s.box == nil {
		return nil, errors.New("boxlite: sandbox not initialized (call Create first)")
	}

	sr, w := schema.Pipe[*filesystem.ExecuteResponse](100)

	st := &streamState{w: w}
	opts := &sdk.ExecutionOptions{
		OnStdout: st.onStdout,
		OnStderr: st.onStderr,
	}
	exec, err := s.box.StartExecution(ctx, "sh", []string{"-c", input.Command}, opts)
	if err != nil {
		go sendErrorAndClose(w, fmt.Errorf("failed to start command: %w", err))
		return sr, nil
	}

	if input.RunInBackendGround {
		st.discardOutput = true
		// Reap in the background; ctx cancellation kills the guest process,
		// mirroring the local backend's background branch.
		go func() {
			done := make(chan struct{})
			go func() {
				_, _ = exec.Wait(context.Background())
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				_ = exec.Kill(context.Background())
			}
		}()
		go func() {
			defer w.Close()
			w.Send(&filesystem.ExecuteResponse{Output: "command started in background\n", ExitCode: new(int)}, nil)
		}()
		return sr, nil
	}

	go func() {
		defer w.Close()
		// Wait's drain barrier guarantees all OnStdout/OnStderr callbacks have
		// flushed before it returns, so the final buffer reads below are safe.
		code, werr := exec.Wait(ctx)
		if werr != nil {
			_ = exec.Kill(context.Background())
			w.Send(nil, werr)
			return
		}
		st.finish(code)
	}()

	return sr, nil
}

// streamState accumulates streaming output: stdout is line-split and sent as
// it arrives; stderr is buffered for the completion message.
type streamState struct {
	w             *schema.StreamWriter[*filesystem.ExecuteResponse]
	discardOutput bool

	mu        sync.Mutex
	lineBuf   []byte
	stderrBuf bytes.Buffer
	hasOutput bool
}

func (st *streamState) onStdout(chunk []byte) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.discardOutput {
		return
	}
	st.lineBuf = append(st.lineBuf, chunk...)
	for {
		i := bytes.IndexByte(st.lineBuf, '\n')
		if i < 0 {
			return
		}
		line := string(st.lineBuf[:i+1])
		st.lineBuf = st.lineBuf[i+1:]
		st.hasOutput = true
		st.w.Send(&filesystem.ExecuteResponse{Output: line}, nil)
	}
}

func (st *streamState) onStderr(chunk []byte) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.discardOutput {
		return
	}
	st.stderrBuf.Write(chunk)
}

// finish flushes any partial last line and sends the completion message.
func (st *streamState) finish(exitCode int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.lineBuf) > 0 {
		st.hasOutput = true
		st.w.Send(&filesystem.ExecuteResponse{Output: string(st.lineBuf)}, nil)
		st.lineBuf = nil
	}
	if exitCode != 0 {
		parts := []string{fmt.Sprintf("command exited with non-zero code %d", exitCode)}
		if st.stderrBuf.Len() > 0 {
			parts = append(parts, "[stderr]:\n"+st.stderrBuf.String())
		}
		st.w.Send(&filesystem.ExecuteResponse{
			Output:   strings.Join(parts, "\n"),
			ExitCode: &exitCode,
		}, nil)
		return
	}
	if !st.hasOutput {
		st.w.Send(&filesystem.ExecuteResponse{ExitCode: new(int)}, nil)
	}
}

// ---- helpers ----

// splitLines splits command output into non-empty lines.
func splitLines(out string) []string {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func sendErrorAndClose(w *schema.StreamWriter[*filesystem.ExecuteResponse], err error) {
	defer w.Close()
	w.Send(nil, err)
}

// teardown best-effort releases a box and its runtime after a failed Create step.
func teardown(ctx context.Context, rt *sdk.Runtime, box *sdk.Box) {
	_ = box.Stop(ctx)
	_ = rt.ForceRemove(ctx, box.ID())
	_ = box.Close()
	_ = rt.Close()
}
