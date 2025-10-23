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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey:     os.Getenv("ARK_API_KEY"),
		Model:      os.Getenv("ARK_MODEL_ID"),
		HTTPClient: &http.Client{Transport: NewCurlLogger(http.DefaultTransport, log.Printf)},
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	streamMsgs, err := chatModel.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	}, ark.WithReasoningEffort(ark.ReasoningEffortLevelMinimal))

	if err != nil {
		log.Printf("Generate failed, err=%v", err)
		return
	}

	defer streamMsgs.Close() // do not forget to close the stream

	msgs := make([]*schema.Message, 0)

	log.Printf("typewriter output:")
	for {
		msg, err := streamMsgs.Recv()
		if err == io.EOF {
			break
		}
		msgs = append(msgs, msg)
		if err != nil {
			log.Printf("\nstream.Recv failed, err=%v", err)
			return
		}
		fmt.Print(msg.Content)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Printf("ConcatMessages failed, err=%v", err)
		return
	}

	log.Printf("output: %s\n", msg.Content)
}

// CurlLogger 是一个 HTTP 中间件，用于将请求转换为 curl 命令并打印出来
type CurlLogger struct {
	// 下一个处理器
	next http.RoundTripper
	// 日志输出函数
	logf func(format string, v ...interface{})
}

// NewCurlLogger 创建一个新的 CurlLogger 中间件
func NewCurlLogger(next http.RoundTripper, logf func(format string, v ...interface{})) *CurlLogger {
	if logf == nil {
		logf = func(format string, v ...interface{}) {
			fmt.Printf(format, v...)
		}
	}
	return &CurlLogger{
		next: next,
		logf: logf,
	}
}

// RoundTrip 实现 http.RoundTripper 接口
func (c *CurlLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	// 生成并打印 curl 命令
	curlCmd := generateCurlCommand(req)
	c.logf("CURL: %s\n", curlCmd)

	// 调用下一个处理器
	return c.next.RoundTrip(req)
}

// generateCurlCommand 将 HTTP 请求转换为等效的 curl 命令
func generateCurlCommand(req *http.Request) string {
	var command strings.Builder

	// 基础 curl 命令
	command.WriteString("curl -X " + req.Method)

	// 添加 URL
	command.WriteString(" '" + req.URL.String() + "'")

	// 添加请求头
	for key, values := range req.Header {
		for _, value := range values {
			command.WriteString(fmt.Sprintf(" -H '%s: %s'", key, value))
		}
	}

	// 添加请求体
	if req.Body != nil && (req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH") {
		var bodyBytes []byte
		// 保存原始请求体
		if req.GetBody != nil {
			bodyReadCloser, err := req.GetBody()
			if err == nil {
				bodyBytes, _ = io.ReadAll(bodyReadCloser)
				bodyReadCloser.Close()
			}
		} else if req.Body != nil {
			// 如果没有 GetBody，则尝试读取 Body，但这会消耗 Body
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body.Close()
			// 重新设置 Body 以便后续处理
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if len(bodyBytes) > 0 {
			bodyStr := string(bodyBytes)
			// 转义单引号
			bodyStr = strings.ReplaceAll(bodyStr, "'", "'\\''")
			command.WriteString(" -d '" + bodyStr + "'")
		}
	}

	return command.String()
}
