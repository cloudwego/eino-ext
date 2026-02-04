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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk/filesystem"
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

type sandboxToolBackend struct {
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

// NewSandboxToolBackend creates a new sandboxToolBackend instance.
// sandboxToolBackend refers to the sandbox running instance created by the sandbox tool in Volcengine.
// For creating a sandbox tool environment, please refer to: https://www.volcengine.com/docs/86681/1847934?lang=zh;
// For creating a sandbox tool running instance, please refer to: https://www.volcengine.com/docs/86681/1860266?lang=zh.
// Note: The execution paths within the sandbox environment may be subject to permission restrictions (read, write, execute, etc.).
// Improper path selection can result in operation failures or permission errors.
// It is recommended to perform operations within paths where the sandbox environment has explicit permissions to mitigate permission-related risks.
func NewSandboxToolBackend(config *Config) (filesystem.Backend, error) {
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

	return &sandboxToolBackend{
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
func (s *sandboxToolBackend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	path, err := formatPath(req.Path, "/", true)
	if err != nil {
		return nil, err
	}

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
func (s *sandboxToolBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (string, error) {
	path, err := formatPath(req.FilePath, "", true)
	if err != nil {
		return "", err
	}
	if req.Offset <= 0 {
		req.Offset = 0
	}
	if req.Limit <= 0 {
		req.Limit = 200
	}

	params := map[string]any{
		"file_path": path,
		"offset":    req.Offset,
		"limit":     req.Limit,
	}

	script, err := pyfmt.Fmt(readPythonCodeTemplate, params)
	if err != nil {
		return "", fmt.Errorf("failed to render read template: %w", err)
	}

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return "", fmt.Errorf("failed to execute read script: %w", err)
	}
	if exitCode != nil && *exitCode != 0 {
		return "", fmt.Errorf("read script exited with non-zero code %d: %s", *exitCode, output)
	}

	return output, nil
}

// GrepRaw searches for content matching the specified pattern in files.
func (s *sandboxToolBackend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	path, _ := formatPath(req.Path, "", false)
	params := map[string]any{
		"pattern":      req.Pattern,
		"path":         path,
		"glob_pattern": req.Glob,
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

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		var match filesystem.GrepMatch
		if err := json.Unmarshal([]byte(line), &match); err != nil {
			// Log or ignore malformed JSON lines
			log.Printf("failed to unmarshal grep match line: %v, line: %s", err, line)
			continue
		}
		matches = append(matches, match)
	}

	return matches, nil
}

// GlobInfo returns file information matching the glob pattern.
func (s *sandboxToolBackend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	path, _ := formatPath(req.Path, "/", false)
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

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		var fi filesystem.FileInfo
		if err := json.Unmarshal([]byte(line), &fi); err != nil {
			continue
		}
		files = append(files, fi)
	}

	return files, nil
}

// Write creates file content.
func (s *sandboxToolBackend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path, err := formatPath(req.FilePath, "", true)
	if err != nil {
		return err
	}

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
func (s *sandboxToolBackend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	path, err := formatPath(req.FilePath, "", true)
	if err != nil {
		return err
	}

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

// execute executes a command in the sandbox.
func (s *sandboxToolBackend) execute(ctx context.Context, command string) (text string, exitCode *int, err error) {
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

func (s *sandboxToolBackend) invokeTool(ctx context.Context, method string, body []byte) ([]byte, error) {
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

func (s *sandboxToolBackend) signRequest(request *http.Request, queries url.Values, body []byte) {
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

func (s *sandboxToolBackend) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (result *filesystem.ExecuteResponse, err error) {
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

	output, exitCode, err := s.execute(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command script: %w", err)
	}

	if exitCode != nil && *exitCode != 0 {
		return nil, fmt.Errorf("command exited with non-zero code %d: %s", *exitCode, output)
	}

	return &filesystem.ExecuteResponse{
		Output: output,
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

// formatPath normalizes a file path with optional default value and absolute path validation.
// If defaultPath is non-empty and path is empty, defaultPath will be used.
// If requireAbs is true, returns an error if the cleaned path is not absolute.
func formatPath(path string, defaultPath string, requireAbs bool) (string, error) {
	if path == "" && defaultPath != "" {
		path = defaultPath
	}
	path = filepath.Clean(path)
	if requireAbs && !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be an absolute path: %s", path)
	}
	return path, nil
}
