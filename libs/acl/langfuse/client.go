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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

func newClient(
	cli *http.Client,
	host string,
	publicKey string,
	secretKey string,
	sdkVersion string,
) *client {
	if cli == nil {
		cli = http.DefaultClient
	}
	return &client{
		cli:        cli,
		host:       host,
		publicKey:  publicKey,
		secretKey:  secretKey,
		sdkVersion: sdkVersion,
	}
}

type client struct {
	cli        *http.Client
	host       string
	publicKey  string
	secretKey  string
	sdkVersion string
}

type apiError struct {
	Status  int
	Message string
	Details any
}

func (e *apiError) Error() string {
	sb := &strings.Builder{}
	sb.WriteString("[\n")
	sb.WriteString(" Status:")
	sb.WriteString(strconv.Itoa(e.Status))
	sb.WriteString("\n Message:")
	sb.WriteString(e.Message)
	if d, ok := e.Details.(string); ok {
		sb.WriteString("\nDetails:")
		sb.WriteString(d)
	}
	sb.WriteString("\n]\n")
	return sb.String()
}

type apiErrors []*apiError

func (b apiErrors) Error() string {
	sb := &strings.Builder{}
	sb.WriteString("API errors: \n")
	for _, e := range b {
		sb.WriteString(e.Error())
	}
	return sb.String()
}

func (c *client) addBaseHeaders(req *http.Request) {
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.publicKey, c.secretKey))))
	req.Header.Add("x_langfuse_public_key", c.publicKey)
	req.Header.Add("x_langfuse_sdk_name", "eino")
	req.Header.Add("x_langfuse_sdk_version", c.sdkVersion)
}

func (c *client) batchIngestion(batch []*event, metadata map[string]string) error {
	body, err := sonic.Marshal(batchIngestionRequest{
		Batch:    batch,
		MetaData: metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal ingestion request body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.host+ingestionPath, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create batch ingestion request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	c.addBaseHeaders(req)

	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do ingestion request: %v", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("failed to close ingestion response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read ingestion response: %v", err)}
	}
	respBody := &batchIngestionResponse{}
	jsonErr := sonic.Unmarshal(b, respBody)
	if jsonErr != nil {
		return &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to unmarshal ingestion response body: %v", jsonErr)}
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	} else if resp.StatusCode == http.StatusMultiStatus {
		if len(respBody.Errors) == 0 {
			return nil
		}
		multiErr := make(apiErrors, 0, len(respBody.Errors))
		for _, e := range respBody.Errors {
			multiErr = append(multiErr, &apiError{
				Status:  e.Status,
				Message: e.Message,
				Details: e.Error,
			})
		}
		return multiErr
	}
	return &apiError{Status: resp.StatusCode, Message: string(b)}
}

func (c *client) getUploadURL(
	m *media,
	traceID string,
	observationID string,
	field fieldType,
) (mediaID string, uploadURL string, err error) {
	body, err := json.Marshal(getUploadURLRequest{
		TraceID:       traceID,
		ObservationID: observationID,
		ContentType:   m.contentType,
		ContentLength: len(m.contentBytes),
		SHA256Hash:    m.contentSHA256Hash,
		Field:         field,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal get upload url request body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.host+getUploadURLPath, bytes.NewBuffer(body))
	if err != nil {
		return "", "", fmt.Errorf("failed to create get upload url request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	c.addBaseHeaders(req)

	resp, err := c.cli.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to do get upload url request: %v", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("failed to close ingestion response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read ingestion response: %v", err)}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		respBody := &getUploadURLResponse{}
		err = sonic.Unmarshal(b, respBody)
		if err != nil {
			return "", "", &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to unmarshal get upload url response body: %v", err)}
		}
		return respBody.MediaID, respBody.UploadURL, nil
	}
	return "", "", &apiError{Status: resp.StatusCode, Message: string(b)}
}

type getUploadURLRequest struct {
	TraceID       string    `json:"traceId,omitempty"`
	ObservationID string    `json:"observationId,omitempty"`
	ContentType   string    `json:"contentType,omitempty"`
	ContentLength int       `json:"contentLength,omitempty"`
	SHA256Hash    string    `json:"sha256Hash,omitempty"`
	Field         fieldType `json:"field,omitempty"`
}

type getUploadURLResponse struct {
	MediaID   string `json:"mediaId"`
	UploadURL string `json:"uploadUrl"`
}

func (c *client) uploadMedia(m *media, uploadURL string) (int, string, error) {
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewBuffer(m.contentBytes))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create upload media request: %w", err)
	}
	req.Header.Add("Content-Type", m.contentType)
	req.Header.Add("x-amz-checksum-sha256", m.contentSHA256Hash)
	req.Header.Add("x-ms-blob-type", "BlockBlob")

	resp, err := c.cli.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("failed to do upload media request: %v", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("failed to close upload media response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read upload media response: %v", err)
	}
	return resp.StatusCode, string(b), nil
}

type patchMediaRequest struct {
	UploadedAt       time.Time `json:"uploadedAt"`
	UploadHTTPStatus int       `json:"uploadHttpStatus"`
	UploadHTTPError  string    `json:"uploadHttpError,omitempty"`
	UploadTimeMs     int64     `json:"uploadTimeMs"`
}

func (c *client) patchMedia(
	mediaID string,
	uploadedAt time.Time,
	uploadHTTPStatus int,
	uploadHTTPError string,
	uploadTimeMs int64,
) error {
	body, err := sonic.Marshal(patchMediaRequest{
		UploadedAt:       uploadedAt,
		UploadHTTPStatus: uploadHTTPStatus,
		UploadHTTPError:  uploadHTTPError,
		UploadTimeMs:     uploadTimeMs,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal patch media request body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf(c.host+patchMediaPath, mediaID), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create patch media request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	c.addBaseHeaders(req)
	resp, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do patch media request: %v", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("failed to close patch media response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return &apiError{Status: resp.StatusCode, Message: fmt.Sprintf("failed to read patch media response: %v", err)}
	}
	if resp.StatusCode < 300 {
		return nil
	}
	return &apiError{Status: resp.StatusCode, Message: string(b)}
}
