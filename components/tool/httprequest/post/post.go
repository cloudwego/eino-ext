package post

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	sonic "github.com/bytedance/sonic"
)

type PostRequest struct {
	URL  string `json:"url" jsonschema_description:"The URL to make the POST request"`
	Body string `json:"body" jsonschema_description:"The body to send in the POST request"`
}

type PostResponse struct {
	Content interface{} `json:"content" jsonschema_description:"The response of the POST request"`
}

func (r *PostRequestTool) Post(ctx context.Context, req *PostRequest) (string, error) {

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.URL, strings.NewReader(req.Body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range r.config.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)
	response := PostResponse{Content: content}
	jsonResp, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to serialize response: %w", err)
	}

	return string(jsonResp), nil
}
