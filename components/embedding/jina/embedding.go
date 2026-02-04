package jina

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

// JinaEmbedder implements the eino Embedder interface for Jina AI
type JinaEmbedder struct {
	config *JinaConfig
	client *http.Client
}

// JinaConfig holds configuration for Jina embedder
type JinaConfig struct {
	APIKey     string        `json:"api_key"`
	Model      string        `json:"model"`
	Task       string        `json:"task"` // e.g., "text-matching", "retrieval.query", "retrieval.passage"
	BaseURL    string        `json:"base_url"`
	Timeout    time.Duration `json:"timeout"`
	Dimensions *int          `json:"dimensions"`
}

// JinaRequest represents the request body for Jina API
type JinaRequest struct {
	Model string                   `json:"model"`
	Task  string                   `json:"task"`
	Input []map[string]interface{} `json:"input"`
}

// JinaResponse represents the response from Jina API
type JinaResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens  int `json:"total_tokens"`
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

// NewEmbedder creates a new Jina embedder instance
func NewEmbedder(ctx context.Context, config *JinaConfig) (*JinaEmbedder, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("jina api key is required")
	}

	if config.Model == "" {
		config.Model = "jina-embeddings-v4"
	}

	if config.Task == "" {
		config.Task = "retrieval.passage"
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.jina.ai/v1"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.Dimensions == nil {
		dim := 2048
		config.Dimensions = &dim
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &JinaEmbedder{
		config: config,
		client: client,
	}, nil
}

// EmbedStrings embeds multiple strings using Jina API
func (j *JinaEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// Convert texts to Jina API format
	input := make([]map[string]interface{}, len(texts))
	for i, text := range texts {
		input[i] = map[string]interface{}{
			"text": text,
		}
	}

	request := JinaRequest{
		Model: j.config.Model,
		Task:  j.config.Task,
		Input: input,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", j.config.BaseURL+"/embeddings", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.config.APIKey)

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jina api error (status %d): %s", resp.StatusCode, string(body))
	}

	var jinaResp JinaResponse
	if err := json.NewDecoder(resp.Body).Decode(&jinaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(jinaResp.Data) != len(texts) {
		return nil, fmt.Errorf("response data length (%d) doesn't match input length (%d)", len(jinaResp.Data), len(texts))
	}

	result := make([][]float64, len(texts))
	for i, data := range jinaResp.Data {
		result[i] = data.Embedding
	}

	return result, nil
}

// EmbedString embeds a single string
func (j *JinaEmbedder) EmbedString(ctx context.Context, text string, opts ...embedding.Option) ([]float64, error) {
	results, err := j.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// GetType returns the component type
func (j *JinaEmbedder) GetType() components.Component {
	return components.ComponentOfEmbedding
}

// GetDimension returns the embedding dimension (if known)
func (j *JinaEmbedder) GetDimension() int {
	if j.config.Dimensions != nil {
		return *j.config.Dimensions
	}
	// Default dimension for jina-embeddings-v4
	return 2048
}

// Close cleans up resources (no-op for HTTP client)
func (j *JinaEmbedder) Close() error {
	return nil
}
