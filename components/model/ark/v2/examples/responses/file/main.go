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

	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	document := &schema.MessageInputFile{}
	if id := os.Getenv("ARK_FILE_ID"); id != "" {
		document.FileID = &id
	} else {
		path := os.Getenv("ARK_FILE_PATH")
		if path == "" {
			return errors.New("set ARK_FILE_PATH or ARK_FILE_ID to a PDF document")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		document.Base64Data, document.MIMEType, document.Name = &encoded, "application/pdf", filepath.Base(path)
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Summarize this PDF briefly."
	}
	reply, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: prompt}, {Type: schema.ChatMessagePartTypeFileURL, File: document},
	}}})
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
