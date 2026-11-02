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

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/file"
)

var (
	kind     = flag.String("kind", "image", "image, audio, video, or file")
	source   = flag.String("source", "base64", "url, base64, file_id, or upload (temporary Files API upload)")
	value    = flag.String("value", "", "URL, local file path, or processed FileID")
	mimeType = flag.String("mime", "", "base64 MIME type; inferred from the local file extension")
)

func mediaPart(ctx context.Context, connection *ark.ChatModelConfig) (part schema.MessageInputPart, cleanup func() error, err error) {
	cleanup = func() error { return nil }
	mediaKind, mediaSource, mediaValue := *kind, *source, *value
	if *fileID != "" {
		if mediaValue != "" {
			return part, cleanup, errors.New("use either -file-id or -value")
		}
		mediaKind, mediaSource, mediaValue = "file", "file_id", *fileID
	}
	if mediaValue == "" {
		return part, cleanup, errors.New("media requires -value or -file-id; see -help")
	}
	switch mediaKind {
	case "image", "audio", "video", "file":
	default:
		return part, cleanup, fmt.Errorf("unsupported -kind %q", mediaKind)
	}
	common := schema.MessagePartCommon{MIMEType: *mimeType}
	filename := ""
	switch mediaSource {
	case "url":
		common.URL = &mediaValue
	case "file_id":
		common.FileID = &mediaValue
	case "base64":
		data, e := os.ReadFile(mediaValue)
		if e != nil {
			return part, cleanup, e
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		common.Base64Data = &encoded
		filename = filepath.Base(mediaValue)
		if common.MIMEType == "" {
			common.MIMEType = mime.TypeByExtension(filepath.Ext(mediaValue))
		}
		if common.MIMEType == "" {
			return part, cleanup, errors.New("set -mime for this local file")
		}
	case "upload":
		id, remove, e := uploadMedia(ctx, connection, mediaValue)
		if e != nil {
			return part, cleanup, e
		}
		common.FileID, cleanup = &id, remove
	default:
		return part, cleanup, fmt.Errorf("unsupported -source %q", mediaSource)
	}
	switch mediaKind {
	case "image":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common, Detail: schema.ImageURLDetailAuto}}
	case "audio":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}}
	case "video":
		video := &schema.MessageInputVideo{MessagePartCommon: common}
		ark.SetInputVideoFPS(video, 1)
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: video}
	case "file":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common, Name: filename}}
	}
	return part, cleanup, nil
}

func uploadMedia(ctx context.Context, connection *ark.ChatModelConfig, path string) (string, func() error, error) {
	var opts []arkruntime.ConfigOption
	if connection.BaseURL != "" {
		opts = append(opts, arkruntime.WithBaseUrl(connection.BaseURL))
	}
	client := arkruntime.NewClientWithApiKey(connection.APIKey, opts...)
	local, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	uploaded, uploadErr := client.UploadFile(ctx, &file.FileCreateRequest{Purpose: file.PurposeUserData}, local)
	closeErr := local.Close()
	if uploadErr != nil {
		return "", nil, errors.Join(uploadErr, closeErr)
	}
	// Delete only the file created by this invocation, including on inference or
	// processing errors. An existing -source file_id is never deleted.
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := client.DeleteFile(cleanupCtx, uploaded.ID); err != nil {
			return fmt.Errorf("delete temporary upload %s: %w", uploaded.ID, err)
		}
		fmt.Println("Deleted temporary upload:", uploaded.ID)
		return nil
	}
	fmt.Println("Uploaded temporary file:", uploaded.ID)
	if closeErr != nil {
		return "", nil, errors.Join(closeErr, cleanup())
	}
	processed, err := client.WaitForFileProcessing(ctx, uploaded.ID, arkruntime.WaitForFileProcessingOptions{})
	if err == nil && processed.Status != file.StatusActive {
		err = fmt.Errorf("uploaded file is not active: %s", processed.Status)
	}
	if err != nil {
		return "", nil, errors.Join(err, cleanup())
	}
	return uploaded.ID, cleanup, nil
}
