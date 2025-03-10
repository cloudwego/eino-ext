package delete

import (
	"context"
	"fmt"
	"io"
	"net/http"

	sonic "github.com/bytedance/sonic"
)

type DeleteRequest struct {
	URL string `json:"url" jsonschema_description:"The URL to make the DELETE request"`
}

type DeleteResponse struct {
	Content interface{} `json:"content" jsonschema_description:"The response of the DELETE request"`
}

func (r *DeleteRequestTool) Delete(ctx context.Context, req *DeleteRequest) (string, error) {

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, req.URL, nil)
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
	response := DeleteResponse{Content: content}
	jsonResp, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to serialize response: %w", err)
	}

	return string(jsonResp), nil
}
