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
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

var (
	mode     = flag.String("mode", "generate", "generate, stream, tools, structured, reasoning, modifiers, multimodal, tool-result, session, prefix, or web-search")
	prompt   = flag.String("prompt", "Hello!", "user message; tools, reasoning, and cache modes demonstrate fixed conversations")
	thinking = flag.String("thinking", "", "enabled, disabled, or auto; defaults to disabled, or enabled in reasoning mode")
	expected = flag.String("expect", "", "fail unless the final answer contains this text")
	timeout  = flag.Duration("timeout", 3*time.Minute, "deadline for the entire example, including follow-up calls")
	fileID   = flag.String("file-id", "", "processed document FileID; shorthand for -kind file -source file_id -value ID")
)

func main() {
	flag.Parse()
	if err := exampleutil.Run(*timeout, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, connection *ark.ChatModelConfig) (err error) {
	config := &ark.ResponsesAPIConfig{APIKey: connection.APIKey, Model: connection.Model, BaseURL: connection.BaseURL}
	selectedThinking := *thinking
	if selectedThinking == "" {
		selectedThinking = "disabled"
		if *mode == "reasoning" {
			selectedThinking = "enabled"
		}
	}
	switch selectedThinking {
	case "enabled", "disabled", "auto":
		config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingMode(selectedThinking))}
	default:
		return fmt.Errorf("unsupported -thinking %q", selectedThinking)
	}
	switch *mode {
	case "generate", "stream", "tools", "modifiers", "multimodal", "tool-result":
	case "reasoning":
		config.EnableReasoningContentPassback = true
	case "session":
		config.SessionCache = &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}
	case "prefix":
	case "structured":
		config.ResponseFormat = &responses.TextFormat{}
		if err = sonic.UnmarshalString(`{"type":"json_schema","name":"result","strict":true,"schema":{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}}`, config.ResponseFormat); err != nil {
			return err
		}
	case "web-search":
		config.EnableToolWebSearch = &responses.WebSearchTool{Limit: responses.NewOptInt64(2)}
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
	needsMedia := *mode == "multimodal" || *mode == "tool-result" || *fileID != "" || *value != ""
	var media *schema.MessageInputPart
	if needsMedia {
		switch *mode {
		case "generate", "stream", "multimodal", "tool-result":
		default:
			return fmt.Errorf("media input is not used by mode %q", *mode)
		}
		part, cleanup, e := mediaPart(ctx, connection)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, cleanup()) }()
		media = &part
	}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	messages := []*schema.Message{schema.UserMessage(*prompt)}
	if media != nil {
		messages[0] = &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: *prompt}, *media,
		}}
	}
	var answer *schema.Message
	streamed := false
	switch *mode {
	case "stream":
		reader, e := cm.Stream(ctx, messages)
		if e != nil {
			return e
		}
		answer, err = collect(reader, true)
		streamed = true
	case "tools":
		answer, err = runTools(ctx, cm)
	case "tool-result":
		answer, err = runToolResult(ctx, cm, *media)
	case "reasoning":
		answer, err = runReasoning(ctx, cm)
	case "modifiers":
		answer, err = runModifiers(ctx, cm, messages)
		streamed = true
	case "session":
		first, e := cm.Generate(ctx, []*schema.Message{schema.UserMessage("Remember the code CACHE_731. Reply OK.")})
		if e != nil {
			return e
		}
		expires, ok := ark.GetCacheExpiration(first)
		if !ok || expires <= time.Now().Unix() {
			return errors.New("response has no usable cache expiration")
		}
		id, _ := ark.GetResponseID(first)
		fmt.Println("Cached response:", id)
		answer, err = cm.Generate(ctx, []*schema.Message{first, schema.UserMessage("What was the code? Reply with the code only.")})
		if err == nil && !strings.Contains(answer.Content, "CACHE_731") {
			return fmt.Errorf("cached context was not recalled: %q", answer.Content)
		}
	case "prefix":
		info, e := cm.CreatePrefixCache(ctx, []*schema.Message{schema.SystemMessage("Remember the code PREFIX_731.")}, 3600)
		if e != nil {
			return e
		}
		fmt.Println("Prefix response:", info.ResponseID)
		answer, err = cm.Generate(ctx, []*schema.Message{schema.UserMessage("What was the code? Reply with the code only.")}, ark.WithCache(&ark.CacheOption{HeadPreviousResponseID: &info.ResponseID}))
		if err == nil && !strings.Contains(answer.Content, "PREFIX_731") {
			return fmt.Errorf("prefix context was not recalled: %q", answer.Content)
		}
	default:
		answer, err = cm.Generate(ctx, messages)
	}
	if err != nil {
		return err
	}
	if answer == nil || strings.TrimSpace(answer.Content) == "" {
		return errors.New("model returned an empty answer")
	}
	if *mode == "structured" {
		var decoded struct {
			Answer *string `json:"answer"`
		}
		if err = sonic.UnmarshalString(answer.Content, &decoded); err != nil {
			return fmt.Errorf("decode structured output: %w", err)
		}
		if decoded.Answer == nil {
			return errors.New("structured output is missing answer")
		}
		fmt.Printf("Decoded answer: %s\n", *decoded.Answer)
	}
	if !streamed {
		fmt.Println(answer.Content)
	}
	printMetadata(answer)
	if *expected != "" && !strings.Contains(answer.Content, *expected) {
		return fmt.Errorf("answer does not contain expected text %q", *expected)
	}
	if *expected != "" {
		fmt.Printf("Verified expected text: %s\n", *expected)
	}
	return nil
}

// Tool argument fragments are decoded only after ConcatMessages. Text and
// reasoning can also be displayed as they arrive without losing final metadata.
func collect(reader *schema.StreamReader[*schema.Message], display bool) (*schema.Message, error) {
	defer reader.Close()
	var messages []*schema.Message
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if display {
			fmt.Print(msg.ReasoningContent, msg.Content)
		}
		messages = append(messages, msg)
	}
	if display {
		fmt.Println()
	}
	return schema.ConcatMessages(messages)
}

func printMetadata(answer *schema.Message) {
	id, _ := ark.GetResponseID(answer)
	fmt.Println("Response:", id)
	if answer.ResponseMeta != nil {
		fmt.Println("Finish reason:", answer.ResponseMeta.FinishReason)
		if answer.ResponseMeta.Usage != nil {
			fmt.Printf("Usage: %+v\n", answer.ResponseMeta.Usage)
		}
	}
}
