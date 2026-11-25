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
	"log"

	"errors"
	"fmt"
	"os"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/file"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) (err error) {

	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	var options []arkruntime.ConfigOption
	if config.BaseURL != "" {
		options = append(options, arkruntime.WithBaseUrl(config.BaseURL))
	}
	client := arkruntime.NewClientWithApiKey(config.APIKey, options...)
	path := os.Getenv("ARK_FILE_PATH")
	if path == "" {
		return errors.New("set ARK_FILE_PATH to a PDF document to upload")
	}
	local, err := os.Open(path)
	if err != nil {
		return err
	}
	defer local.Close()
	uploaded, err := client.UploadFile(ctx, &file.FileCreateRequest{Purpose: file.PurposeUserData}, local)
	if err != nil {
		return err
	}
	// Clean up only the file created by this example, even when inference fails.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, cleanupErr := client.DeleteFile(cleanupCtx, uploaded.ID); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("delete upload %s: %w", uploaded.ID, cleanupErr))
		} else {
			fmt.Println("Deleted temporary upload:", uploaded.ID)
		}
	}()
	fmt.Println("Uploaded temporary file:", uploaded.ID)
	processed, err := client.WaitForFileProcessing(ctx, uploaded.ID, arkruntime.WaitForFileProcessingOptions{})
	if err != nil {
		return err
	}
	if processed.Status != file.StatusActive {
		return errors.New("uploaded file did not become active")
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Summarize this PDF briefly."
	}
	reply, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: prompt},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &uploaded.ID}}},
	}}})
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
