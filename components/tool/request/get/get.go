package get

import (
	"context"
	"fmt"
	"io"
	"net/http"

	sonic "github.com/bytedance/sonic"
)

type GetRequest struct {
	URL string `json:"url" jsonschema_description:"The URL to make the GET request"`
}

type GetResponse struct {
	Content interface{} `json:"content" jsonschema_description:"The response of the GET request"`
}

func (r *GetRequestTool) Get(ctx context.Context, req *GetRequest) (string, error) {

	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
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

	var content interface{}
	if r.config.ResponseContentType == "json" {
		if err := sonic.Unmarshal(body, &content); err != nil {
			return "", fmt.Errorf("failed to deserialize JSON response: %w", err)
		}
	} else {
		content = string(body)
	}

	response := GetResponse{Content: content}
	jsonResp, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to serialize response: %w", err)
	}

	return string(jsonResp), nil
}
