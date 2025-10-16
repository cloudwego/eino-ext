/*
 * Copyright 2024 CloudWeGo Authors
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

package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	promptsPath      = "/api/public/v2/prompts"
	promptByNamePath = "/api/public/v2/prompts/"
)

type ListMetadata struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// ChatMessageWithPlaceHolder represents a chat message that can include placeholders for dynamic content.
//
// Placeholders in the content can be replaced with actual values when using the prompt.
// The Role field specifies the message role (e.g., "system", "user", "assistant"),
// Type specifies the content type, and Content contains the message text with optional placeholders.
type ChatMessageWithPlaceHolder struct {
	Role    string `json:"role"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func (c *ChatMessageWithPlaceHolder) validate() error {
	if c.Role == "" {
		return errors.New("'role' is required")
	}
	if c.Content == "" {
		return errors.New("'content' is required")
	}
	return nil
}

// PromptEntry represents a complete prompt template with its configuration and messages.
//
// A prompt entry contains the prompt name, which can be either a string (when Type is "text")
// or an array of chat messages with placeholders (for other types).
// The Type field determines the expected structure of the Prompt field.
// The Config field can contain model-specific configuration parameters.
type PromptEntry struct {
	Name    string   `json:"name"`
	Prompt  any      `json:"prompt"`
	Type    string   `json:"type"`
	Version int      `json:"version,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Config  any      `json:"config,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshalling for PromptEntry.
// It correctly unmarshal the Prompt field as either a string (for "text" type)
// or []ChatMessageWithPlaceHolder (for other types) based on the Type field.
func (p *PromptEntry) UnmarshalJSON(data []byte) error {
	// Define an alias to avoid infinite recursion during unmarshalling
	type Alias PromptEntry

	// First, unmarshal into a temporary struct with raw JSON for the Prompt field
	temp := &struct {
		*Alias
		Prompt json.RawMessage `json:"prompt"`
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}

	if p.Type == "text" {
		var promptStr string
		if err := json.Unmarshal(temp.Prompt, &promptStr); err != nil {
			return fmt.Errorf("failed to unmarshal prompt as string for type 'text': %w", err)
		}
		p.Prompt = promptStr
	} else {
		var promptMessages []ChatMessageWithPlaceHolder
		if err := json.Unmarshal(temp.Prompt, &promptMessages); err != nil {
			return fmt.Errorf("failed to unmarshal prompt as []ChatMessageWithPlaceHolder for type '%s': %w", p.Type, err)
		}
		p.Prompt = promptMessages
	}

	return nil
}

func (p *PromptEntry) validate() error {
	if p.Name == "" {
		return errors.New("'name' is required")
	}
	if p.Prompt == nil {
		return errors.New("'prompt' cannot be nil")
	}

	// Validate based on Type field
	if p.Type == "text" {
		// For text type, prompt should be a string
		if str, ok := p.Prompt.(string); !ok || str == "" {
			return errors.New("'prompt' must be a non-empty string when type is 'text'")
		}
	} else {
		// For other types, prompt should be []ChatMessageWithPlaceHolder
		messages, ok := p.Prompt.([]ChatMessageWithPlaceHolder)
		if !ok {
			return errors.New("'prompt' must be []ChatMessageWithPlaceHolder when type is not 'text'")
		}
		if len(messages) == 0 {
			return errors.New("'prompt' cannot be empty")
		}
		for _, msg := range messages {
			if err := msg.validate(); err != nil {
				return fmt.Errorf("invalid prompts message: %w", err)
			}
		}
	}
	return nil
}

// ListParams defines the query parameters for filtering and paginating prompt listings.
//
// Use these parameters to filter prompts by name, labels, tags, and update timestamps,
// as well as to control pagination with Page and Limit fields.
type ListParams struct {
	Name          string
	Label         string
	Tag           string
	Page          int
	Limit         int
	FromUpdatedAt time.Time
	ToUpdatedAt   time.Time
}

// ToQueryString converts the ListParams to a URL query string.
func (query *ListParams) ToQueryString() string {
	parts := make([]string, 0)
	if query.Name != "" {
		parts = append(parts, "name="+query.Name+"")
	}
	if query.Label != "" {
		parts = append(parts, "label="+query.Label)
	}
	if query.Tag != "" {
		parts = append(parts, "tag="+query.Tag)
	}
	if query.Page != 0 {
		parts = append(parts, "page="+strconv.Itoa(query.Page))
	}
	if query.Limit != 0 {
		parts = append(parts, "limit="+strconv.Itoa(query.Limit))
	}
	if !query.FromUpdatedAt.IsZero() {
		// format with ios8601
		parts = append(parts, "fromUpdatedAt="+query.FromUpdatedAt.Format(time.RFC3339))
	}
	if !query.ToUpdatedAt.IsZero() {
		parts = append(parts, "toUpdatedAt="+query.ToUpdatedAt.Format(time.RFC3339))
	}
	return strings.Join(parts, "&")
}

// GetParams defines the parameters for retrieving a specific prompt.
//
// Use Name to specify the prompt name, Label for a specific label,
// and Version for a specific version. If Version is 0, the latest version is returned.
type GetParams struct {
	Name    string
	Label   string
	Version int
}

type PromptMeta struct {
	Name          string    `json:"name"`
	Labels        []string  `json:"labels"`
	Tags          []string  `json:"tags"`
	Versions      []int     `json:"versions"`
	LastConfig    any       `json:"lastConfig,omitempty"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

// ListPrompts represents the response structure for prompt listing operations.
//
// It contains pagination metadata and an array of prompt entries matching the query parameters.
type ListPrompts struct {
	Metadata ListMetadata `json:"meta"`
	Data     []PromptMeta `json:"data"`
}

// PromptClient provides methods for interacting with the Langfuse prompts API.
//
// The client handles HTTP communication with the Langfuse API for prompt management
// operations including creating, retrieving, and listing prompt templates.
type PromptClient struct {
	cli *client
}

// GetPrompt retrieves a specific prompt by name, version, and label.
func (c *PromptClient) GetPrompt(ctx context.Context, params GetParams) (*PromptEntry, error) {
	if params.Name == "" {
		return nil, errors.New("'name' is required")
	}

	url := c.cli.host + promptByNamePath + params.Name

	// Build query parameters
	queryParts := make([]string, 0)
	if params.Version > 0 {
		queryParts = append(queryParts, "version="+strconv.Itoa(params.Version))
	}
	if params.Label != "" {
		queryParts = append(queryParts, "label="+params.Label)
	}
	if len(queryParts) > 0 {
		url += "?" + strings.Join(queryParts, "&")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get prompt request: %w", err)
	}

	c.cli.addBaseHeaders(req)

	resp, err := c.cli.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do get prompt request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read get prompt response: %v", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{Status: resp.StatusCode, Message: string(body)}
	}

	var prompt PromptEntry
	if err := json.Unmarshal(body, &prompt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal get prompt response: %w", err)
	}

	return &prompt, nil
}

// ListPrompt retrieves a list of prompts based on the provided parameters.
func (c PromptClient) ListPrompt(ctx context.Context, params ListParams) (*ListPrompts, error) {
	url := c.cli.host + promptsPath

	queryString := params.ToQueryString()
	if queryString != "" {
		url += "?" + queryString
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list prompts request: %w", err)
	}

	c.cli.addBaseHeaders(req)

	resp, err := c.cli.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do list prompts request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read list prompts response: %v", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{Status: resp.StatusCode, Message: string(body)}
	}

	var listResponse ListPrompts
	if err := json.Unmarshal(body, &listResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list prompts response: %w", err)
	}

	return &listResponse, nil
}

// CreatePrompt creates a new prompt.
func (c *PromptClient) CreatePrompt(ctx context.Context, createPrompt *PromptEntry) (*PromptEntry, error) {
	if err := createPrompt.validate(); err != nil {
		return nil, err
	}

	// For reset the prompt version because it's not supported in the creating API.
	createPrompt.Version = 0

	body, err := json.Marshal(createPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create prompt request body: %w", err)
	}

	url := c.cli.host + promptsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create create prompt request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	c.cli.addBaseHeaders(req)

	resp, err := c.cli.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do create prompt request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read create prompt response: %v", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{Status: resp.StatusCode, Message: string(respBody)}
	}

	var createdPrompt PromptEntry
	if err := json.Unmarshal(respBody, &createdPrompt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create prompt response: %w", err)
	}

	return &createdPrompt, nil
}
