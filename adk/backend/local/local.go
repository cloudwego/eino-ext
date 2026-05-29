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

package local

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
	"github.com/gen2brain/go-fitz"
)

const defaultRootPath = "/"

const (
	defaultMaxImageSizeMB        = 10
	defaultMaxPDFSizeMB          = 20
	defaultMaxPagedPDFSizeMB     = 100
	defaultMaxPDFPagesPerRequest = 20

	// defaultPDFRenderDPI: see MultiModalReadConfig.PDFRenderDPI for the
	// readability vs payload-size trade-off (kept as the single source of truth).
	defaultPDFRenderDPI = 150.0

	// maxConfigurableMB caps any user-supplied MB value. Combined with int64 size
	// arithmetic in readAllBytes, the cap ensures `int64(MB) * 1024 * 1024` never
	// overflows on any platform.
	maxConfigurableMB = 2048
	// maxConfigurablePDFPagesPerRequest hard-caps page count to avoid unbounded
	// memory pressure when rasterising. Tune via MultiModalReadConfig.
	maxConfigurablePDFPagesPerRequest = 1000
	// maxConfigurablePDFRenderDPI bounds DPI to a sane print-grade ceiling.
	maxConfigurablePDFRenderDPI = 600.0
)

// MultiModalReadConfig configures runtime limits for Local.MultiModalRead.
// Any field left at its zero (or negative) value falls back to the package default.
// Values exceeding the package hard-caps are silently clamped to those caps to
// keep size/page math safe (see maxConfigurable* constants).
type MultiModalReadConfig struct {
	// MaxImageSizeMB caps the size of a single image read. Default 10. Hard-cap 2048.
	MaxImageSizeMB int
	// MaxPDFSizeMB caps the size of a full PDF read (no 'pages' param). Default 20. Hard-cap 2048.
	MaxPDFSizeMB int
	// MaxPagedPDFSizeMB caps the size of a paged PDF read (with 'pages' param). Default 100. Hard-cap 2048.
	MaxPagedPDFSizeMB int
	// MaxPDFPagesPerRequest caps the number of pages rendered per paged read. Default 20. Hard-cap 1000.
	MaxPDFPagesPerRequest int
	// PDFRenderDPI is dots-per-inch used when rasterizing each PDF page to PNG.
	// Higher DPI yields sharper images at the cost of larger payloads:
	// typical screens are 72-96 DPI, 150 DPI ≈ 2x sharpness with manageable size,
	// 300 DPI is print-grade but produces ~4x larger images.
	// Default 150. Hard-cap 600.
	PDFRenderDPI float64
}

// resolveMultiModalReadConfig fills any zero/negative field of cfg with the
// corresponding package default, then clamps each field to its hard-cap. The
// returned config has every field guaranteed > 0 and ≤ the hard-cap.
func resolveMultiModalReadConfig(cfg MultiModalReadConfig) MultiModalReadConfig {
	cfg.MaxImageSizeMB = clampInt(cfg.MaxImageSizeMB, defaultMaxImageSizeMB, maxConfigurableMB)
	cfg.MaxPDFSizeMB = clampInt(cfg.MaxPDFSizeMB, defaultMaxPDFSizeMB, maxConfigurableMB)
	cfg.MaxPagedPDFSizeMB = clampInt(cfg.MaxPagedPDFSizeMB, defaultMaxPagedPDFSizeMB, maxConfigurableMB)
	cfg.MaxPDFPagesPerRequest = clampInt(cfg.MaxPDFPagesPerRequest, defaultMaxPDFPagesPerRequest, maxConfigurablePDFPagesPerRequest)
	cfg.PDFRenderDPI = clampFloat(cfg.PDFRenderDPI, defaultPDFRenderDPI, maxConfigurablePDFRenderDPI)
	return cfg
}

// clampInt returns def when v <= 0, max when v > max, otherwise v.
func clampInt(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// clampFloat returns def when v <= 0, max when v > max, otherwise v.
func clampFloat(v, def, max float64) float64 {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// errReadAllBytesTooLarge signals that readAllBytes rejected a file because its
// size exceeded the caller-supplied maxBytes. Use errors.Is to detect it and
// wrap with additional context (e.g. suggesting the 'pages' parameter for PDFs).
var errReadAllBytesTooLarge = errors.New("file exceeds max allowed size")

type Config struct {
	ValidateCommand func(string) error

	// MultiModalRead overrides default size/page/DPI limits used by
	// Local.MultiModalRead. Optional; zero-value fields fall back to
	// package defaults (see MultiModalReadConfig field comments).
	MultiModalRead MultiModalReadConfig
}

type Local struct {
	validateCommand func(string) error

	// multiModalReadCfg carries already-resolved (defaults applied, hard-caps
	// enforced) limits used by MultiModalRead. Every field is guaranteed > 0.
	multiModalReadCfg MultiModalReadConfig
}

var defaultValidateCommand = func(string) error {
	return nil
}

// NewBackend creates a new local filesystem Local instance.
//
// IMPORTANT - System Compatibility:
//   - Supported: Unix/MacOS only
//   - NOT Supported: Windows (requires custom implementation of filesystem.Backend)
//   - Command Execution: Uses /bin/sh by default for Execute method
//   - If /bin/sh does not meet your requirements, please implement your own filesystem.Backend
func NewBackend(_ context.Context, cfg *Config) (*Local, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	validateCommand := defaultValidateCommand
	if cfg.ValidateCommand != nil {
		validateCommand = cfg.ValidateCommand
	}

	return &Local{
		validateCommand:   validateCommand,
		multiModalReadCfg: resolveMultiModalReadConfig(cfg.MultiModalRead),
	}, nil
}

func (s *Local) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	path := filepath.Clean(req.Path)
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", path)
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []filesystem.FileInfo
	for _, entry := range entries {
		files = append(files, filesystem.FileInfo{
			Path: entry.Name(),
		})
	}

	return files, nil
}

func (s *Local) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	path := filepath.Clean(req.FilePath)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.Size() == 0 {
		return &filesystem.FileContent{}, nil
	}

	offset := req.Offset
	if offset <= 0 {
		offset = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 2000
	}

	reader := bufio.NewReader(file)
	var result strings.Builder
	lineNum := 1
	linesRead := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if line != "" {
			if lineNum >= offset {
				result.WriteString(line)
				linesRead++
				if linesRead >= limit {
					break
				}
			}
			lineNum++
		}
		if err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf("error reading file: %w", err)
			}
			break
		}
	}

	return &filesystem.FileContent{
		Content: strings.TrimSuffix(result.String(), "\n"),
	}, nil
}

// MultiModalRead reads file content with multimodal support for images and PDFs.
// For non-image/non-PDF files, it delegates to the standard Read method.
//
// Limits and DPI are configurable via Config.MultiModalRead and enforced on the
// Go side. Defaults:
//   - image: 10 MB
//   - PDF full read (no 'pages' param): 20 MB
//   - PDF paged read (with 'pages' param): 100 MB, max 20 pages per request
//   - PDF render DPI: 150
//
// For paged PDF reads, if the requested end page exceeds the actual total pages,
// it is silently clamped to the last page. For example, requesting pages "1-100"
// on a 5-page PDF returns pages 1 through 5.
//
// PDF rendering relies on go-fitz (MuPDF via purego/ffi, no classic CGO).
// If build fails due to missing MuPDF libs, install them:
//   - macOS:  brew install mupdf
//   - Linux(Ubuntu/Debian): apt-get install -y libmupdf-dev
//   - Linux(CentOS/RHEL):   yum install -y mupdf-devel
func (s *Local) MultiModalRead(ctx context.Context, req *filesystem.MultiModalReadRequest) (*filesystem.MultiFileContent, error) {
	path := filepath.Clean(req.FilePath)
	ext := strings.ToLower(filepath.Ext(path))

	// If the file is not an image or PDF, delegate to the standard Read method.
	if !isImageExt(ext) && !isPDFExt(ext) {
		content, err := s.Read(ctx, &req.ReadRequest)
		if err != nil {
			return nil, err
		}
		return &filesystem.MultiFileContent{
			FileContent: content,
		}, nil
	}
	// Image branch.
	if isImageExt(ext) {
		maxImageSizeMB := s.multiModalReadCfg.MaxImageSizeMB
		data, err := s.readAllBytes(ctx, path, int64(maxImageSizeMB)*1024*1024)
		if err != nil {
			if errors.Is(err, errReadAllBytesTooLarge) {
				return nil, fmt.Errorf("%w; image size limit is %dMB, please compress or downsample the image before reading", err, maxImageSizeMB)
			}
			return nil, fmt.Errorf("failed to read file bytes: %w", err)
		}
		mime := detectImageMIME(data)
		if mime == "" {
			return nil, fmt.Errorf("file %s has image extension but content is not a recognized image format", path)
		}
		return &filesystem.MultiFileContent{
			Parts: []filesystem.FileContentPart{newImageContentPart(mime, data)},
		}, nil
	}

	// PDF branch — fail fast on offline validations before reading bytes or opening the doc.
	paged := req.Pages != ""
	var pagedStart, pagedEnd int
	if paged {
		var err error
		pagedStart, pagedEnd, err = parsePagesParam(req.Pages, s.multiModalReadCfg.MaxPDFPagesPerRequest)
		if err != nil {
			return nil, err
		}
	}

	maxPDFSizeMB := s.multiModalReadCfg.MaxPDFSizeMB
	maxPagedPDFSizeMB := s.multiModalReadCfg.MaxPagedPDFSizeMB
	sizeLimit := int64(maxPDFSizeMB) * 1024 * 1024
	if paged {
		sizeLimit = int64(maxPagedPDFSizeMB) * 1024 * 1024
	}
	data, err := s.readAllBytes(ctx, path, sizeLimit)
	if err != nil {
		if errors.Is(err, errReadAllBytesTooLarge) {
			if paged {
				return nil, fmt.Errorf("%w; paged PDF size limit is %dMB, the file is too large even for paged reading", err, maxPagedPDFSizeMB)
			}
			return nil, fmt.Errorf("%w; PDF full-read size limit is %dMB, use the 'pages' parameter to read page ranges (limit raised to %dMB)", err, maxPDFSizeMB, maxPagedPDFSizeMB)
		}
		return nil, fmt.Errorf("failed to read file bytes: %w", err)
	}

	if !isPDFBytes(data) {
		return nil, fmt.Errorf("file %s has .pdf extension but content is not a valid PDF", path)
	}

	doc, err := fitz.NewFromMemory(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF %s: %w", path, err)
	}
	defer doc.Close()

	totalPages := doc.NumPage()
	if totalPages == 0 {
		return nil, fmt.Errorf("file %s has 0 pages, cannot read", path)
	}

	if paged {
		if pagedStart > totalPages {
			return nil, fmt.Errorf("invalid pages parameter: %q (start page %d exceeds total pages %d in file %s)", req.Pages, pagedStart, totalPages, path)
		}
		if pagedEnd > totalPages {
			pagedEnd = totalPages
		}
		parts, err := renderPDFPagesToImages(ctx, doc, pagedStart, pagedEnd, path, s.multiModalReadCfg.PDFRenderDPI)
		if err != nil {
			return nil, err
		}
		return &filesystem.MultiFileContent{Parts: parts}, nil
	}

	return &filesystem.MultiFileContent{
		Parts: []filesystem.FileContentPart{
			{
				Type:     filesystem.FileContentPartTypePDF,
				MIMEType: "application/pdf",
				Data:     data,
			},
		},
	}, nil
}

// readAllBytes reads the entire file at path from the local filesystem,
// rejecting it up-front when its size exceeds maxBytes.
//
// maxBytes is int64 so callers can pass GiB-scale limits without worrying about
// platform-dependent int width.
//
// Size enforcement has two layers:
//   - Primary check: os.Stat compares Size() against maxBytes before any
//     allocation, so oversize files never enter memory.
//   - Defense-in-depth: after os.ReadFile, the decoded length is re-checked in
//     case the file grew between Stat and ReadFile.
func (s *Local) readAllBytes(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", path)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("%w: file %s (%d bytes, limit %dMB)", errReadAllBytesTooLarge, path, info.Size(), maxBytes/1024/1024)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: file %s (%d bytes, limit %dMB)", errReadAllBytesTooLarge, path, len(data), maxBytes/1024/1024)
	}
	return data, nil
}

// parsePagesParam parses and validates the pages parameter format.
// It only enforces syntax rules and the per-request page-count ceiling
// (maxPages); it does NOT know about the actual PDF page count, so callers
// must clamp against totalPages after opening the document.
//
// Supported formats:
//   - "1"   → single page
//   - "1-3" → inclusive range
//
// Open-ended ranges like "1-" and multi-range strings like "1-2-3" are
// rejected. Returned start, end are 1-based inclusive.
//
// All errors are uniformly prefixed with `invalid pages parameter: %q (...)`
// so callers can surface a single, recognizable error pattern.
//
// Defensive: a non-positive maxPages is silently replaced with the package
// default, so misuse from internal callers cannot produce a "limit 0" false
// positive that rejects every valid range.
func parsePagesParam(pages string, maxPages int) (start, end int, err error) {
	if maxPages <= 0 {
		maxPages = defaultMaxPDFPagesPerRequest
	}
	startStr, endStr, hasRange, err := splitPagesRange(pages)
	if err != nil {
		return 0, 0, err
	}

	start, err = strconv.Atoi(startStr)
	if err != nil || start < 1 {
		return 0, 0, fmt.Errorf("invalid pages parameter: %q (start page must be a positive integer)", pages)
	}

	if !hasRange {
		return start, start, nil
	}

	end, err = strconv.Atoi(endStr)
	if err != nil || end < 1 {
		return 0, 0, fmt.Errorf("invalid pages parameter: %q (end page must be a positive integer)", pages)
	}

	if err := validatePagesRange(start, end, maxPages, pages); err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// splitPagesRange splits the raw pages string by '-' and handles whitespace,
// empty input, multi-range input, and the open-ended case.
func splitPagesRange(pages string) (startStr, endStr string, hasRange bool, err error) {
	trimmed := strings.TrimSpace(pages)
	if trimmed == "" {
		return "", "", false, fmt.Errorf("invalid pages parameter: %q (empty)", pages)
	}
	if strings.Count(trimmed, "-") > 1 {
		return "", "", false, fmt.Errorf("invalid pages parameter: %q (only a single range is supported, e.g. \"1-5\")", pages)
	}
	parts := strings.SplitN(trimmed, "-", 2)
	startStr = strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return startStr, "", false, nil
	}
	endStr = strings.TrimSpace(parts[1])
	if endStr == "" {
		return "", "", false, fmt.Errorf("invalid pages parameter: %q (open-ended range is not supported, end page is required)", pages)
	}
	return startStr, endStr, true, nil
}

// validatePagesRange enforces the business rules for a parsed [start, end]
// range: end must not precede start, and the inclusive length must fit within
// maxPages. totalPages-based clamping is a caller concern.
func validatePagesRange(start, end, maxPages int, pages string) error {
	if end < start {
		return fmt.Errorf("invalid pages parameter: %q (end page %d < start page %d)", pages, end, start)
	}
	if end-start+1 > maxPages {
		return fmt.Errorf("invalid pages parameter: %q (range spans %d pages, exceeds limit %d)", pages, end-start+1, maxPages)
	}
	return nil
}

// renderPDFPagesToImages converts the specified page range [start, end] (1-based)
// from the PDF document to PNG images at the given DPI. A non-positive dpi is
// silently replaced with the default to defend against misuse from future
// internal callers. The loop checks ctx between pages so callers can cancel
// long-running renders.
func renderPDFPagesToImages(ctx context.Context, doc *fitz.Document, start, end int, path string, dpi float64) ([]filesystem.FileContentPart, error) {
	if dpi <= 0 {
		dpi = defaultPDFRenderDPI
	}
	count := end - start + 1
	parts := make([]filesystem.FileContentPart, 0, count)
	for i := start - 1; i < end; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		pngData, err := doc.ImagePNG(i, dpi)
		if err != nil {
			return nil, fmt.Errorf("failed to convert page %d to image for %s: %w", i+1, path, err)
		}
		parts = append(parts, newImageContentPart("image/png", pngData))
	}
	return parts, nil
}

// newImageContentPart builds a FileContentPart with image type and the given
// MIME type and payload.
func newImageContentPart(mime string, data []byte) filesystem.FileContentPart {
	return filesystem.FileContentPart{
		Type:     filesystem.FileContentPartTypeImage,
		MIMEType: mime,
		Data:     data,
	}
}

// isImageExt checks if the file extension represents an image.
func isImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif":
		return true
	}
	return false
}

// isPDFExt checks if the file extension represents a PDF.
func isPDFExt(ext string) bool {
	return ext == ".pdf"
}

// detectImageMIME detects the MIME type from image file bytes using magic
// number headers. Returns the MIME type string or empty string if not a
// recognized image. Each branch guards its own minimum length so new formats
// added later don't have to rely on a shared top-level length check.
func detectImageMIME(data []byte) string {
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	// JPEG: FF D8 FF
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}
	// GIF: GIF87a or GIF89a
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return "image/gif"
	}
	// BMP: BM
	if len(data) >= 2 && bytes.Equal(data[:2], []byte("BM")) {
		return "image/bmp"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	// TIFF: 49 49 2A 00 (little-endian) or 4D 4D 00 2A (big-endian)
	if len(data) >= 4 && (bytes.Equal(data[:4], []byte{0x49, 0x49, 0x2A, 0x00}) || bytes.Equal(data[:4], []byte{0x4D, 0x4D, 0x00, 0x2A})) {
		return "image/tiff"
	}
	return ""
}

// isPDFBytes checks if the data starts with the PDF magic number (%PDF-).
func isPDFBytes(data []byte) bool {
	return len(data) >= 5 && bytes.Equal(data[:5], []byte("%PDF-"))
}

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

func (s *Local) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	path := filepath.Clean(req.Path)

	cmd := []string{"rg", "--json"}
	if req.CaseInsensitive {
		cmd = append(cmd, "-i")
	}
	if req.EnableMultiline {
		cmd = append(cmd, "-U", "--multiline-dotall")
	}
	if req.FileType != "" {
		cmd = append(cmd, "--type", req.FileType)
	} else if req.Glob != "" {
		cmd = append(cmd, "--glob", req.Glob)
	}
	if req.AfterLines > 0 {
		cmd = append(cmd, "-A", fmt.Sprintf("%d", req.AfterLines))
	}
	if req.BeforeLines > 0 {
		cmd = append(cmd, "-B", fmt.Sprintf("%d", req.BeforeLines))
	}

	cmd = append(cmd, "-e", req.Pattern, "--", path)

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	output, err := execCmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("ripgrep (rg) is not installed or not in PATH. Please install it: https://github.com/BurntSushi/ripgrep#installation")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return []filesystem.GrepMatch{}, nil
			}
			return nil, fmt.Errorf("ripgrep failed with exit code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute ripgrep: %w", err)
	}

	var matches []filesystem.GrepMatch
	if len(output) == 0 {
		return matches, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var data rgJSON
	for _, line := range lines {
		data = rgJSON{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data.Type == "match" || data.Type == "context" {
			matchPath := data.Data.Path.Text
			if req.FileType != "" && req.Glob != "" {
				matched, _ := doublestar.Match(req.Glob, matchPath)
				if !matched {
					matched, _ = doublestar.Match(req.Glob, filepath.Base(matchPath))
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
	}

	return matches, nil
}

func (s *Local) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if req.Path == "" {
		req.Path = defaultRootPath
	}
	path := filepath.Clean(req.Path)

	var matches []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return err
		}

		relPath, err := filepath.Rel(path, p)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		matched, _ := doublestar.Match(req.Pattern, relPath)
		if matched {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	sort.Strings(matches)

	var files []filesystem.FileInfo
	for _, match := range matches {
		files = append(files, filesystem.FileInfo{
			Path: match,
		})
	}

	return files, nil
}

func (s *Local) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path := filepath.Clean(req.FilePath)

	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(req.Content))
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (s *Local) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	path := filepath.Clean(req.FilePath)
	if req.OldString == "" {
		return fmt.Errorf("old string is required")
	}

	if req.OldString == req.NewString {
		return fmt.Errorf("new string must be different from old string")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)
	count := strings.Count(text, req.OldString)

	if count == 0 {
		return fmt.Errorf("string not found in file: '%s'", req.OldString)
	}
	if count > 1 && !req.ReplaceAll {
		return fmt.Errorf("string '%s' appears multiple times. Use replace_all=True to replace all occurrences", req.OldString)
	}

	var newText string
	if req.ReplaceAll {
		newText = strings.Replace(text, req.OldString, req.NewString, -1)
	} else {
		newText = strings.Replace(text, req.OldString, req.NewString, 1)
	}

	return os.WriteFile(path, []byte(newText), 0644)
}

func (s *Local) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (result *schema.StreamReader[*filesystem.ExecuteResponse], err error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}

	cmd, stdout, stderr, err := s.initStreamingCmd(ctx, input.Command)
	if err != nil {
		return nil, err
	}

	sr, w := schema.Pipe[*filesystem.ExecuteResponse](100)

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		go sendErrorAndClose(w, fmt.Errorf("failed to start command: %w", err))
		return sr, nil
	}

	if input.RunInBackendGround {
		s.runCmdInBackground(ctx, cmd, stdout, stderr, w)
		return sr, nil
	}

	go s.streamCmdOutput(ctx, cmd, stdout, stderr, w)

	return sr, nil
}

func (s *Local) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (result *filesystem.ExecuteResponse, err error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", input.Command)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
			stdoutStr := stdoutBuf.String()
			stderrStr := stderrBuf.String()
			parts := []string{fmt.Sprintf("command exited with non-zero code %d", exitCode)}
			if stdoutStr != "" {
				parts = append(parts, "[stdout]:\n"+strings.TrimSuffix(stdoutStr, ""))
			}
			if stderrStr != "" {
				parts = append(parts, "[stderr]:\n"+strings.TrimSuffix(stderrStr, ""))
			}
			return &filesystem.ExecuteResponse{
				Output:   strings.Join(parts, "\n"),
				ExitCode: &exitCode,
			}, nil
		}
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	return &filesystem.ExecuteResponse{
		Output:   stdoutBuf.String(),
		ExitCode: &exitCode,
	}, nil
}

// initStreamingCmd creates command with stdout and stderr pipes.
func (s *Local) initStreamingCmd(ctx context.Context, command string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return cmd, stdout, stderr, nil
}

// runCmdInBackground executes command in background without waiting for completion.
// The caller controls timeout/cancellation via ctx.Done().
func (s *Local) runCmdInBackground(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				_ = cmd.Process.Kill()
			}
			_ = stdout.Close()
			_ = stderr.Close()
		}()

		done := make(chan struct{})
		go func() {
			drainPipesConcurrently(stdout, stderr)
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		}
	}()

	go func() {
		defer w.Close()
		w.Send(&filesystem.ExecuteResponse{Output: "command started in background\n", ExitCode: new(int)}, nil)
	}()
}

// drainPipesConcurrently consumes stdout and stderr concurrently to prevent pipe blocking.
func drainPipesConcurrently(stdout, stderr io.Reader) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stderr)
	}()
	wg.Wait()
}

// streamCmdOutput handles streaming command output to the writer.
func (s *Local) streamCmdOutput(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	defer func() {
		if pe := recover(); pe != nil {
			w.Send(nil, newPanicErr(pe, debug.Stack()))
			return
		}
		w.Close()
	}()

	stderrData, stderrErr := s.readStderrAsync(stderr)

	hasOutput, err := s.streamStdout(ctx, cmd, stdout, w)
	if err != nil {
		w.Send(nil, err)
		return
	}

	if stdError := <-stderrErr; stdError != nil {
		w.Send(nil, stdError)
		return
	}

	s.handleCmdCompletion(cmd, stderrData, hasOutput, w)
}

// readStderrAsync reads stderr in a separate goroutine.
func (s *Local) readStderrAsync(stderr io.Reader) (*[]byte, <-chan error) {
	stderrData := new([]byte)
	stderrErr := make(chan error, 1)

	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				stderrErr <- newPanicErr(pe, debug.Stack())
				return
			}
			close(stderrErr)
		}()
		var err error
		*stderrData, err = io.ReadAll(stderr)
		if err != nil {
			stderrErr <- fmt.Errorf("failed to read stderr: %w", err)
		}
	}()

	return stderrData, stderrErr
}

// streamStdout streams stdout line by line to the writer.
func (s *Local) streamStdout(ctx context.Context, cmd *exec.Cmd, stdout io.Reader, w *schema.StreamWriter[*filesystem.ExecuteResponse]) (bool, error) {
	reader := bufio.NewReader(stdout)
	hasOutput := false

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			hasOutput = true
			select {
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				return hasOutput, ctx.Err()
			default:
				w.Send(&filesystem.ExecuteResponse{Output: line}, nil)
			}
		}
		if err != nil {
			if err != io.EOF {
				return hasOutput, fmt.Errorf("error reading stdout: %w", err)
			}
			break
		}
	}

	return hasOutput, nil
}

// handleCmdCompletion handles command completion and sends final response.
func (s *Local) handleCmdCompletion(cmd *exec.Cmd, stderrData *[]byte, hasOutput bool, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode := exitError.ExitCode()
			parts := []string{fmt.Sprintf("command exited with non-zero code %d", exitCode)}
			if stderrStr := string(*stderrData); stderrStr != "" {
				parts = append(parts, "[stderr]:\n"+stderrStr)
			}
			w.Send(&filesystem.ExecuteResponse{
				Output:   strings.Join(parts, "\n"),
				ExitCode: &exitCode,
			}, nil)
			return
		}

		w.Send(nil, fmt.Errorf("command failed: %w", err))
		return
	}

	if !hasOutput {
		w.Send(&filesystem.ExecuteResponse{ExitCode: new(int)}, nil)
	}
}

// sendErrorAndClose sends an error to the stream and closes it.
func sendErrorAndClose(w *schema.StreamWriter[*filesystem.ExecuteResponse], err error) {
	defer w.Close()
	w.Send(nil, err)
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
