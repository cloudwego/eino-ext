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

package boxlite

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
)

// --- Unit tests: need the BoxLite native library to link, but no hypervisor.

func TestNewBackendAppliesDefaults(t *testing.T) {
	s, err := NewBackend(context.Background(), nil)
	if err != nil {
		t.Fatalf("NewBackend(nil): %v", err)
	}
	if s.config.Image != defaultImage {
		t.Errorf("Image = %q, want %q", s.config.Image, defaultImage)
	}
	if s.config.WorkDir != defaultWorkDir {
		t.Errorf("WorkDir = %q, want %q", s.config.WorkDir, defaultWorkDir)
	}
	if s.config.CPUs != defaultCPUs {
		t.Errorf("CPUs = %d, want %d", s.config.CPUs, defaultCPUs)
	}
	if s.config.MemoryMiB != defaultMemoryMiB {
		t.Errorf("MemoryMiB = %d, want %d", s.config.MemoryMiB, defaultMemoryMiB)
	}
	if s.config.FileOpTimeout != defaultFileOpTimeout {
		t.Errorf("FileOpTimeout = %v, want %v", s.config.FileOpTimeout, defaultFileOpTimeout)
	}
}

func TestResolvePath(t *testing.T) {
	s, _ := NewBackend(context.Background(), nil)

	if got := s.resolvePath(""); got != defaultWorkDir {
		t.Errorf("resolvePath(\"\") = %q, want %q", got, defaultWorkDir)
	}
	if got := s.resolvePath("a/b.txt"); got != defaultWorkDir+"/a/b.txt" {
		t.Errorf("resolvePath(rel) = %q, want %q", got, defaultWorkDir+"/a/b.txt")
	}
	if got := s.resolvePath("/etc/hosts"); got != "/etc/hosts" {
		t.Errorf("resolvePath(abs) = %q, want %q", got, "/etc/hosts")
	}
}

// --- E2E: boot a real microVM. Opt-in via BOXLITE_E2E=1 (needs the native lib
// and a hypervisor: KVM on linux/amd64, the Hypervisor entitlement on
// darwin/arm64). Override the image with BOXLITE_TEST_IMAGE.

func bootE2E(t *testing.T) *Sandbox {
	t.Helper()
	if os.Getenv("BOXLITE_E2E") == "" {
		t.Skip("set BOXLITE_E2E=1 to run the microVM e2e tests")
	}
	ctx := context.Background()
	cfg := &Config{}
	if img := os.Getenv("BOXLITE_TEST_IMAGE"); img != "" {
		cfg.Image = img
	}
	s, err := NewBackend(ctx, cfg)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	if err := s.Create(ctx); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { s.Cleanup(ctx) })
	return s
}

func TestE2E_WriteReadEditLsGlob(t *testing.T) {
	ctx := context.Background()
	s := bootE2E(t)

	const body = "alpha\nbeta with $pecial `chars`\ngamma\n"
	if err := s.Write(ctx, &filesystem.WriteRequest{FilePath: "sub/dir/file.txt", Content: body}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read(ctx, &filesystem.ReadRequest{FilePath: "sub/dir/file.txt"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := strings.TrimSuffix(body, "\n"); got.Content != want {
		t.Errorf("Read = %q, want %q", got.Content, want)
	}

	// Line-window read: offset 2, limit 1 -> only the second line.
	win, err := s.Read(ctx, &filesystem.ReadRequest{FilePath: "sub/dir/file.txt", Offset: 2, Limit: 1})
	if err != nil {
		t.Fatalf("Read window: %v", err)
	}
	if want := "beta with $pecial `chars`"; win.Content != want {
		t.Errorf("Read window = %q, want %q", win.Content, want)
	}

	if _, err := s.Read(ctx, &filesystem.ReadRequest{FilePath: "sub/dir/missing.txt"}); err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Read missing = %v, want file-not-found error", err)
	}

	if err := s.Edit(ctx, &filesystem.EditRequest{FilePath: "sub/dir/file.txt", OldString: "gamma", NewString: "delta"}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	got2, err := s.Read(ctx, &filesystem.ReadRequest{FilePath: "sub/dir/file.txt"})
	if err != nil {
		t.Fatalf("Read after Edit: %v", err)
	}
	if !strings.Contains(got2.Content, "delta") || strings.Contains(got2.Content, "gamma") {
		t.Errorf("Edit did not replace: %q", got2.Content)
	}
	if err := s.Edit(ctx, &filesystem.EditRequest{FilePath: "sub/dir/file.txt", OldString: "nope", NewString: "x"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Edit missing old = %v, want not-found error", err)
	}

	ls, err := s.LsInfo(ctx, &filesystem.LsInfoRequest{Path: "sub/dir"})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	if len(ls) != 1 || ls[0].Path != "file.txt" {
		t.Errorf("LsInfo = %+v, want [file.txt]", ls)
	}
	if ls2, err := s.LsInfo(ctx, &filesystem.LsInfoRequest{Path: "no/such/dir"}); err != nil || ls2 != nil {
		t.Errorf("LsInfo missing = %+v,%v, want nil,nil", ls2, err)
	}

	glob, err := s.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: "**/*.txt", Path: "sub"})
	if err != nil {
		t.Fatalf("GlobInfo: %v", err)
	}
	if len(glob) != 1 || glob[0].Path != "dir/file.txt" {
		t.Errorf("GlobInfo = %+v, want [dir/file.txt]", glob)
	}
}

func TestE2E_WriteLarge(t *testing.T) {
	ctx := context.Background()
	s := bootE2E(t)

	// 300 KiB spans multiple base64 chunks, exercising the chunked-append path.
	large := strings.Repeat("0123456789abcdef", 300*1024/16)
	if err := s.Write(ctx, &filesystem.WriteRequest{FilePath: "big.bin", Content: large}); err != nil {
		t.Fatalf("Write large: %v", err)
	}
	res, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "wc -c < big.bin"})
	if err != nil {
		t.Fatalf("Execute wc: %v", err)
	}
	if got := strings.TrimSpace(res.Output); got != "307200" {
		t.Errorf("guest file size = %s, want 307200", got)
	}
}

func TestE2E_Execute(t *testing.T) {
	ctx := context.Background()
	s := bootE2E(t)

	res, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "echo hello-from-vm && sleep 0.1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := strings.TrimSpace(res.Output); got != "hello-from-vm" {
		t.Errorf("Output = %q, want hello-from-vm", got)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", res.ExitCode)
	}

	// Non-zero exit is a result, not an error, in the local backend's format.
	res2, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "echo boom >&2; exit 7"})
	if err != nil {
		t.Fatalf("Execute non-zero: %v", err)
	}
	if res2.ExitCode == nil || *res2.ExitCode != 7 {
		t.Errorf("ExitCode = %v, want 7", res2.ExitCode)
	}
	if !strings.Contains(res2.Output, "non-zero code 7") || !strings.Contains(res2.Output, "boom") {
		t.Errorf("Output = %q, want combined non-zero report", res2.Output)
	}
}

func TestE2E_ExecuteStreaming(t *testing.T) {
	ctx := context.Background()
	s := bootE2E(t)

	sr, err := s.ExecuteStreaming(ctx, &filesystem.ExecuteRequest{Command: "for i in 1 2 3; do echo line$i; done; sleep 0.1"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	var lines []string
	for {
		resp, rerr := sr.Recv()
		if rerr != nil {
			break // io.EOF ends the stream
		}
		if resp.Output != "" {
			lines = append(lines, strings.TrimSuffix(resp.Output, "\n"))
		}
	}
	joined := strings.Join(lines, "|")
	for _, want := range []string{"line1", "line2", "line3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("stream missing %q: %q", want, joined)
		}
	}
}

func TestE2E_GrepRaw(t *testing.T) {
	ctx := context.Background()
	s := bootE2E(t)

	// The GrepRaw contract requires ripgrep in the guest image. When the test
	// image doesn't ship it, assert the friendly install hint, then skip.
	probe, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "command -v rg"})
	if err != nil {
		t.Fatalf("probe rg: %v", err)
	}
	if probe.ExitCode == nil || *probe.ExitCode != 0 {
		if _, gerr := s.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "x", Path: "."}); gerr == nil || !strings.Contains(gerr.Error(), "not installed") {
			t.Fatalf("GrepRaw without rg = %v, want install-hint error", gerr)
		}
		t.Skip("guest image has no ripgrep; GrepRaw requires rg in the image")
	}

	if err := s.Write(ctx, &filesystem.WriteRequest{FilePath: "src/a.go", Content: "package a\nfunc Hello() {}\n"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	matches, err := s.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "func\\s+Hello", Path: "src"})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if len(matches) != 1 || matches[0].Line != 2 {
		t.Errorf("GrepRaw = %+v, want one match on line 2", matches)
	}
}
