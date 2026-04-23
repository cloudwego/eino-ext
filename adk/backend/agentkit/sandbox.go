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

package agentkit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/gen2brain/go-fitz"
	"github.com/slongfield/pyfmt"
)

type Region string

const (
	service                 = "agentkit"
	regionOfBeijingBaseURL  = "https://agentkit.cn-beijing.volces.com"
	regionOfShangHaiBaseURL = "https://agentkit.cn-shanghai.volces.com"
	python3KernelName       = "python3"
	runCodeOperationType    = "RunCode"
)

const (
	RegionOfBeijing  Region = "cn-beijing"
	RegionOfShangHai Region = "cn-shanghai"
)

const (
	maxImageSizeMB    = 10
	maxPDFSizeMB      = 20
	maxPagedPDFSizeMB = 100

	maxImageSize    = maxImageSizeMB * 1024 * 1024
	maxPDFSize      = maxPDFSizeMB * 1024 * 1024
	maxPagedPDFSize = maxPagedPDFSizeMB * 1024 * 1024

	maxPDFPagesPerRequest = 20

	// defaultPDFRenderDPI: 150 DPI balances readability and file size — typical screen is 72-96 DPI,
	// 150 gives ~2x sharpness while keeping PNG sizes manageable for API transport.
	defaultPDFRenderDPI = 150.0
)

// errReadAllBytesTooLarge signals that readAllBytes rejected a file because its
// size exceeded the caller-supplied maxBytes. Use errors.Is to detect it and
// wrap with additional context (e.g. suggesting the 'pages' parameter for PDFs).
var errReadAllBytesTooLarge = errors.New("file exceeds max allowed size")

// tooLargeMarker is the unique sentinel prefix the Python side prints to stdout
// when it rejects a file by size. Go-side matches on this prefix instead of the
// exit code because exit-code numbers are not reserved and collide with
// unrelated Python/system errors.
//
// Safety: the success path writes base64 ([A-Za-z0-9+/=]) only, which cannot
// start with "__", so the marker cannot collide with a valid payload.
const tooLargeMarker = "__READALLBYTES_TOO_LARGE__"

// Config holds the configuration for the Ark Sandbox.
type Config struct {
	AccessKeyID string

	SecretAccessKey string

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// Region is the request's region, e.g., cn-beijing. This parameter should be set to the
	// actual region you want to access when using a product that provides services by region.
	// Optional. Default: cn-beijing
	Region Region

	// ToolID is the ID of the sandbox tool.
	// Required.
	ToolID string

	// SessionID specifies the session ID for the execution request.
	// If the parameter is provided but empty, a new session will be created.
	// Note: Since the SessionID becomes unavailable when the tool's lifecycle ends,
	// it is recommended to use UserSessionID for execution requests.
	// If neither SessionID nor UserSessionID is provided, a new UserSessionID will be created by default.
	// Optional.
	SessionID string

	// UserSessionID specifies the user session information for the execution request.
	// This field can be used to specify the session instance for the execution request to achieve context isolation.
	// If the parameter is provided with a value: the request is executed according to the incoming session information.
	// If the session information does not exist, a new session will be created.
	// If the parameter is provided but empty: a new session will be created.
	// For more details, see: https://www.volcengine.com/docs/86681/2155980
	// Note: If neither SessionID nor UserSessionID is provided, a new UserSessionID will be created by default.
	// Optional.
	UserSessionID string

	// SessionTTL is the time-to-live for the session instance in seconds.
	// The valid range is 60-86400.
	// This field only takes effect when creating a new session instance.
	// For more details, see: https://www.volcengine.com/docs/86681/2155980
	// Optional. Default 1800.
	SessionTTL int

	// ExecutionTimeout is the timeout for code execution in the sandbox instance.
	// Unit: seconds.
	// For more details, see: https://www.volcengine.com/docs/86681/2155980
	ExecutionTimeout int
}

type SandboxTool struct {
	secretAccessKey  string
	accessKeyID      string
	baseURL          string
	region           Region
	httpClient       *http.Client
	toolID           string
	userSessionID    string
	sessionID        string
	sessionTTL       int
	executionTimeout int
}

// NewSandboxToolBackend creates a new SandboxTool instance.
// SandboxTool refers to the sandbox running instance created by the sandbox tool in Volcengine.
// For creating a sandbox tool environment, please refer to: https://www.volcengine.com/docs/86681/1847934?lang=zh;
// For creating a sandbox tool running instance, please refer to: https://www.volcengine.com/docs/86681/1860266?lang=zh.
// Note: The execution paths within the sandbox environment may be subject to permission restrictions (read, write, execute, etc.).
// Improper path selection can result in operation failures or permission errors.
// It is recommended to perform operations within paths where the sandbox environment has explicit permissions to mitigate permission-related risks.
func NewSandboxToolBackend(config *Config) (*SandboxTool, error) {
	if config.AccessKeyID == "" {
		return nil, fmt.Errorf("AccessKeyID is required")
	}
	if config.SecretAccessKey == "" {
		return nil, fmt.Errorf("SecretAccessKey is required")
	}
	if config.ToolID == "" {
		return nil, fmt.Errorf("ToolID is required")
	}

	if config.SessionID == "" && config.UserSessionID == "" {
		return nil, fmt.Errorf("SessionID or UserSessionID is required, at least one must be provided")
	}

	httpClient := http.DefaultClient
	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	}

	region := config.Region
	if region == "" {
		region = RegionOfBeijing
	}

	var baseURL string
	switch region {
	case RegionOfBeijing:
		baseURL = regionOfBeijingBaseURL
	case RegionOfShangHai:
		baseURL = regionOfShangHaiBaseURL
	default:
		return nil, fmt.Errorf("invalid region: %s", region)
	}

	return &SandboxTool{
		accessKeyID:      config.AccessKeyID,
		secretAccessKey:  config.SecretAccessKey,
		httpClient:       httpClient,
		region:           region,
		baseURL:          baseURL,
		toolID:           config.ToolID,
		sessionID:        config.SessionID,
		userSessionID:    config.UserSessionID,
		sessionTTL:       config.SessionTTL,
		executionTimeout: config.ExecutionTimeout,
	}, nil
}

// LsInfo lists file information under the given path.
func (s *SandboxTool) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	path := filepath.Clean(req.Path)

	params := map[string]any{
		"path": path,
	}

	script, err := pyfmt.Fmt(lsInfoPythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render ls template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute ls script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return nil, fmt.Errorf("ls script exited with non-zero code %d: %s", *exitCode, output)
	}

	var files []filesystem.FileInfo
	if output == "" {
		return files, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		var fi filesystem.FileInfo
		if err := json.Unmarshal([]byte(line), &fi); err != nil {
			// Ignore lines that can't be unmarshalled
			continue
		}
		files = append(files, fi)
	}

	return files, nil
}

// Read reads file content with support for line-based offset and limit.
func (s *SandboxTool) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	path := filepath.Clean(req.FilePath)
	if req.Offset <= 0 {
		req.Offset = 1
	}

	if req.Limit <= 0 {
		req.Limit = 2000
	}

	params := map[string]any{
		"file_path": path,
		"offset":    req.Offset,
		"limit":     req.Limit,
	}

	script, err := pyfmt.Fmt(readPythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render read template: %w", err)
	}

	content, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute read script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return nil, fmt.Errorf("read script exited with non-zero code %d: %s", *exitCode, content)
	}

	return &filesystem.FileContent{
		Content: content,
	}, nil
}

// GrepRaw searches for content matching the specified pattern in files.
func (s *SandboxTool) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	path := filepath.Clean(req.Path)
	params := map[string]any{
		"fileType":    req.FileType,
		"glob":        req.Glob,
		"afterLines":  req.AfterLines,
		"beforeLines": req.BeforeLines,
		"pattern":     req.Pattern,
		"path":        path,
	}
	if req.CaseInsensitive {
		params["caseInsensitive"] = 1
	} else {
		params["caseInsensitive"] = 0
	}
	if req.EnableMultiline {
		params["enableMultiline"] = 1
	} else {
		params["enableMultiline"] = 0
	}

	script, err := pyfmt.Fmt(grepPythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render grep template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute grep script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return nil, fmt.Errorf("grep script exited with code %d: %s", *exitCode, output)
	}

	var matches []filesystem.GrepMatch
	if output == "" {
		return matches, nil
	}
	err = json.Unmarshal([]byte(output), &matches)
	if err != nil {
		return nil, fmt.Errorf("failed to parse grep output: %w", err)
	}

	return matches, nil
}

// GlobInfo returns file information matching the glob pattern.
func (s *SandboxTool) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	path := filepath.Clean(req.Path)
	params := map[string]any{
		"path_b64":    base64.StdEncoding.EncodeToString([]byte(path)),
		"pattern_b64": base64.StdEncoding.EncodeToString([]byte(req.Pattern)),
	}

	script, err := pyfmt.Fmt(globPythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render glob template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute glob script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return nil, fmt.Errorf("glob script exited with non-zero code %d: %s", *exitCode, output)
	}

	var files []filesystem.FileInfo
	if output == "" {
		return files, nil
	}

	err = json.Unmarshal([]byte(output), &files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse glob output: %w", err)
	}
	return files, nil

}

// Write creates file content.
func (s *SandboxTool) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path := filepath.Clean(req.FilePath)

	params := map[string]any{
		"file_path":   path,
		"content_b64": base64.StdEncoding.EncodeToString([]byte(req.Content)),
	}

	script, err := pyfmt.Fmt(writePythonCodeTemplate, params)
	if err != nil {
		return fmt.Errorf("failed to render write template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to execute write script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return fmt.Errorf("write script exited with non-zero code %d: %s", *exitCode, output)
	}

	return nil
}

// Edit replaces string occurrences in a file.
func (s *SandboxTool) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	path := filepath.Clean(req.FilePath)

	if req.OldString == "" {
		return fmt.Errorf("old string is required")
	}

	if req.OldString == req.NewString {
		return fmt.Errorf("new string must be different from old string")
	}

	replaceAll := 1
	if !req.ReplaceAll {
		replaceAll = 0
	}
	params := map[string]any{
		"file_path":   path,
		"old_b64":     base64.StdEncoding.EncodeToString([]byte(req.OldString)),
		"new_b64":     base64.StdEncoding.EncodeToString([]byte(req.NewString)),
		"replace_all": replaceAll,
	}

	script, err := pyfmt.Fmt(editPythonCodeTemplate, params)
	if err != nil {
		return fmt.Errorf("failed to render edit template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to execute edit script: %w", err)
	}

	if exitCode != nil && *exitCode != 0 {
		return fmt.Errorf("edit script exited with non-zero code %d: %s", *exitCode, output)
	}

	return nil
}

// MultiModalRead reads file content with multimodal support for images and PDFs.
// For non-image/non-PDF files, it delegates to the standard Read method.
//
// Size limits (enforced both in the Python sandbox and on the Go side):
//   - image: 10 MB (maxImageSize)
//   - PDF full read (no 'pages' param): 20 MB (maxPDFSize)
//   - PDF paged read (with 'pages' param): 100 MB (maxPagedPDFSize), max 20 pages per request (maxPDFPagesPerRequest)
//
// PDF rendering relies on go-fitz (MuPDF via purego/ffi, no classic CGO).
// If build fails due to missing MuPDF libs, install them:
//   - macOS:  brew install mupdf
//   - Linux(Ubuntu/Debian): apt-get install -y libmupdf-dev
//   - Linux(CentOS/RHEL):   yum install -y mupdf-devel
func (s *SandboxTool) MultiModalRead(ctx context.Context, req *filesystem.MultiModalReadRequest) (*filesystem.MultiFileContent, error) {
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
		data, err := s.readAllBytes(ctx, path, maxImageSize)
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
		pagedStart, pagedEnd, err = parsePagesParam(req.Pages)
		if err != nil {
			return nil, err
		}
	}

	sizeLimit := maxPDFSize
	if paged {
		sizeLimit = maxPagedPDFSize
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
			return nil, fmt.Errorf("start page %d exceeds total page count %d (valid range: 1-%d) for file %s (pages=%q); adjust the 'pages' parameter accordingly", pagedStart, totalPages, totalPages, path, req.Pages)
		}
		// Keep clamp to allow read-to-end style requests like "1-100" on short PDFs;
		// surface a warn so the caller can notice the mismatch.
		if pagedEnd > totalPages {
			log.Printf("[WARN] MultiModalRead: end page %d exceeds total pages %d for %s (pages=%q), clamped to %d", pagedEnd, totalPages, path, req.Pages, totalPages)
			pagedEnd = totalPages
		}
		parts, err := renderPDFPagesToImages(doc, pagedStart, pagedEnd, path)
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

// parsePagesParam parses and validates the pages parameter format.
// It only enforces syntax rules and the per-request page-count ceiling
// (maxPDFPagesPerRequest); it does NOT know about the actual PDF page count,
// so callers must clamp against totalPages after opening the document.
//
// Supported formats:
//   - "1"   → single page
//   - "1-3" → inclusive range
//
// Open-ended ranges like "1-" are rejected; an explicit end page is required.
// Returned start, end are 1-based inclusive.
func parsePagesParam(pages string) (start, end int, err error) {
	startStr, endStr, hasRange, err := splitPagesRange(pages)
	if err != nil {
		return 0, 0, err
	}

	start, err = strconv.Atoi(startStr)
	if err != nil || start < 1 {
		return 0, 0, fmt.Errorf("invalid start page in pages parameter: %q", pages)
	}

	if !hasRange {
		return start, start, nil
	}

	end, err = strconv.Atoi(endStr)
	if err != nil || end < 1 {
		return 0, 0, fmt.Errorf("invalid end page in pages parameter: %q", pages)
	}

	if err := validatePagesRange(start, end, pages); err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// splitPagesRange splits the raw pages string by '-' and handles whitespace
// plus the empty/open-ended cases. It does not parse numbers.
func splitPagesRange(pages string) (startStr, endStr string, hasRange bool, err error) {
	pages = strings.TrimSpace(pages)
	if pages == "" {
		return "", "", false, fmt.Errorf("pages parameter is empty")
	}
	parts := strings.SplitN(pages, "-", 2)
	startStr = strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return startStr, "", false, nil
	}
	endStr = strings.TrimSpace(parts[1])
	if endStr == "" {
		return "", "", false, fmt.Errorf("open-ended page range is not supported, please specify an end page (max %d pages per request)", maxPDFPagesPerRequest)
	}
	return startStr, endStr, true, nil
}

// validatePagesRange enforces the business rules for a parsed [start, end] range:
// end must not precede start, and the inclusive length must fit within
// maxPDFPagesPerRequest. totalPages-based clamping is a caller concern.
func validatePagesRange(start, end int, pages string) error {
	if end < start {
		return fmt.Errorf("end page %d is less than start page %d in pages parameter: %q", end, start, pages)
	}
	if end-start+1 > maxPDFPagesPerRequest {
		return fmt.Errorf("requested %d pages (%d-%d) exceeds the limit of %d pages per request", end-start+1, start, end, maxPDFPagesPerRequest)
	}
	return nil
}

// renderPDFPagesToImages converts the specified page range [start, end] (1-based)
// from the PDF data to PNG images and returns them as FileContentParts.
func renderPDFPagesToImages(doc *fitz.Document, start, end int, path string) ([]filesystem.FileContentPart, error) {
	count := end - start + 1
	parts := make([]filesystem.FileContentPart, 0, count)
	for i := start - 1; i < end; i++ {
		pngData, err := doc.ImagePNG(i, defaultPDFRenderDPI)
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

// readAllBytes reads all bytes of the file at the given path from the sandbox.
//
// Size enforcement has two layers:
//   - Primary check (Python side): before reading, the script compares the
//     file size against maxBytes. If exceeded, it emits the tooLargeMarker
//     prefix and exits non-zero, so oversize payloads never traverse the
//     sandbox API response in the normal path.
//   - Defense-in-depth (Go side): after decoding, the decoded length is
//     re-checked. This branch is unreachable when the Python script behaves
//     correctly, and exists only as a safety net against a buggy or tampered
//     script.
func (s *SandboxTool) readAllBytes(ctx context.Context, path string, maxBytes int) ([]byte, error) {
	params := map[string]any{
		"file_path_b64": base64.StdEncoding.EncodeToString([]byte(path)),
		"max_bytes":     maxBytes,
	}

	script, err := pyfmt.Fmt(readAllBytesPythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render readAllBytes template: %w", err)
	}

	content, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute readAllBytes script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		if strings.HasPrefix(content, tooLargeMarker) {
			return nil, fmt.Errorf("%w: file %s (limit %dMB)", errReadAllBytesTooLarge, path, maxBytes/1024/1024)
		}
		return nil, fmt.Errorf("readAllBytes script exited with non-zero code %d: %s", *exitCode, content)
	}

	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 content: %w", err)
	}

	if len(data) > maxBytes {
		return nil, fmt.Errorf("%w: file %s (%d bytes, limit %dMB)", errReadAllBytesTooLarge, path, len(data), maxBytes/1024/1024)
	}

	return data, nil
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

// detectImageMIME detects the MIME type from image file bytes using magic number headers.
// Returns the MIME type string or empty string if not a recognized image.
// Each branch guards its own minimum length so new formats added later don't
// have to rely on a shared top-level length check.
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

// execute executes a command in the sandbox.
func (s *SandboxTool) execute(ctx context.Context, command string) (text string, exitCode *int, err error) {
	var operationPayload string
	if s.executionTimeout <= 0 {
		operationPayload, err = sonic.MarshalString(map[string]any{
			"code":       command,
			"kernelName": python3KernelName,
		})
	} else {
		operationPayload, err = sonic.MarshalString(map[string]any{
			"code":       command,
			"timeout":    s.executionTimeout,
			"kernelName": python3KernelName,
		})
	}

	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal operation payload: %w", err)
	}

	req := &invokeToolRequest{
		ToolID:           s.toolID,
		SessionID:        s.sessionID,
		UserSessionID:    s.userSessionID,
		OperationPayload: operationPayload,
		OperationType:    runCodeOperationType,
	}

	if s.sessionTTL > 0 {
		req.Ttl = &s.sessionTTL
	}

	requestBytes, err := sonic.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	respBody, err := s.invokeTool(ctx, http.MethodPost, requestBytes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to invoke tool: %w", err)
	}

	var resp response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var ret result
	if err := json.Unmarshal([]byte(resp.Result.Result), &ret); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal result data: %w", err)
	}

	if !ret.Success {
		errorExitCode := -1
		if len(ret.Data.Outputs) > 0 {
			firstOutput := ret.Data.Outputs[0]
			if firstOutput.Text != "" {
				text = firstOutput.Text
			} else if firstOutput.EName != "" {
				text = fmt.Sprintf("%s: %s", firstOutput.EName, firstOutput.EValue)
			}
		}
		return text, &errorExitCode, nil
	}

	exitCode = new(int) // Success, so exit code is 0
	if len(ret.Data.Outputs) > 0 {
		text = ret.Data.Outputs[0].Text
	}

	return text, exitCode, nil
}

func (s *SandboxTool) invokeTool(ctx context.Context, method string, body []byte) ([]byte, error) {
	queries := make(url.Values)
	queries.Set("Action", "InvokeTool")
	queries.Set("Version", "2025-10-30")
	requestAddr := fmt.Sprintf("%s%s?%s", s.baseURL, "/", queries.Encode())

	request, err := http.NewRequestWithContext(ctx, method, requestAddr, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("bad request: %w", err)
	}

	s.signRequest(request, queries, body)

	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("do request err: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status code %d", response.StatusCode)
	}

	return responseBody, nil
}

func (s *SandboxTool) signRequest(request *http.Request, queries url.Values, body []byte) {
	now := time.Now()
	date := now.UTC().Format("20060102T150405Z")
	authDate := date[:8]
	request.Header.Set("X-Date", date)

	payload := hex.EncodeToString(hashSHA256(body))
	request.Header.Set("X-Content-Sha256", payload)
	request.Header.Set("Content-Type", "application/json")

	queryString := strings.Replace(queries.Encode(), "+", "%20", -1)
	signedHeaders := []string{"host", "x-date", "x-content-sha256", "content-type"}
	var headerList []string
	for _, header := range signedHeaders {
		if header == "host" {
			headerList = append(headerList, header+":"+request.Host)
		} else {
			v := request.Header.Get(header)
			headerList = append(headerList, header+":"+strings.TrimSpace(v))
		}
	}
	headerString := strings.Join(headerList, "\n")

	canonicalString := strings.Join([]string{
		request.Method,
		"/",
		queryString,
		headerString + "\n",
		strings.Join(signedHeaders, ";"),
		payload,
	}, "\n")

	hashedCanonicalString := hex.EncodeToString(hashSHA256([]byte(canonicalString)))

	credentialScope := authDate + "/" + string(s.region) + "/" + service + "/request"
	signString := strings.Join([]string{
		"HMAC-SHA256",
		date,
		credentialScope,
		hashedCanonicalString,
	}, "\n")

	signedKey := getSignedKey(s.secretAccessKey, authDate, string(s.region), service)
	signature := hex.EncodeToString(hmacSHA256(signedKey, signString))

	authorization := "HMAC-SHA256" +
		" Credential=" + s.accessKeyID + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature
	request.Header.Set("Authorization", authorization)
}

func (s *SandboxTool) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (result *filesystem.ExecuteResponse, err error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	params := map[string]any{
		"command_b64": base64.StdEncoding.EncodeToString([]byte(input.Command)),
	}

	script, err := pyfmt.Fmt(executePythonCodeTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("failed to render execute template: %w", err)
	}

	if input.RunInBackendGround {
		go func() {
			_, _, _ = s.execute(ctx, script)
		}()
		return &filesystem.ExecuteResponse{
			Output: "command started in background\n",
		}, nil
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command script: %w", err)
	}

	return &filesystem.ExecuteResponse{
		Output:   output,
		ExitCode: exitCode,
	}, nil
}

func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func getSignedKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte(secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "request")
	return kSigning
}

func hashSHA256(data []byte) []byte {
	hash := sha256.New()
	if _, err := hash.Write(data); err != nil {
		log.Printf("input hash err:%s", err.Error())
	}
	return hash.Sum(nil)
}
