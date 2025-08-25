/*
 * Copyright 2025 CloudWeGo Authors
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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/a2a/client"
	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
)

func main() {
	ctx := context.Background()
	transport, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:       "http://localhost:8888",
		HandlerPath:   "/test",
		AgentCardPath: nil,
		HertzClient:   nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	cli, err := client.NewA2AClient(ctx, &client.Config{
		Transport: transport,
	})
	if err != nil {
		log.Fatal(err)
	}

	card, err := cli.AgentCard(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("AgentCard: %+v", card)

	fmt.Printf("\n\n>>>>>>>>>>base chat<<<<<<<<<<\n\n")
	baseChat(ctx, cli)
	fmt.Printf("\n\n>>>>>>>>>stream chat<<<<<<<<<\n\n")
	streamChat(ctx, cli)
	fmt.Printf("\n\n>>>>>>>>>resubscribe<<<<<<<<<\n\n")
	resubscribeChat(ctx, cli)
	fmt.Printf("\n\n>>>>>>>>>>>notify<<<<<<<<<<<<\n\n")
	notifyChat(ctx, cli)
}

func baseChat(ctx context.Context, cli *client.A2AClient) {
	result, err := cli.SendMessage(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("hello")},
			},
		},
		Configuration: nil,
		Metadata:      nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("first chat:")
	fmt.Println(printTask(result.Task))
	taskID := result.Task.ID
	result, err = cli.SendMessage(ctx, &models.MessageSendParams{
		Message: models.Message{
			TaskID: &taskID,
			Role:   models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("hello2")},
			},
		},
		Configuration: nil,
		Metadata:      nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("second chat:")
	fmt.Println(printTask(result.Task))

	queryResult, err := cli.GetTask(ctx, &models.TaskQueryParams{
		ID: taskID,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("query result:")
	fmt.Println(printTask(queryResult))
}

func streamChat(ctx context.Context, cli *client.A2AClient) {
	stream, err := cli.SendMessageStreaming(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("hello")},
			},
		},
		Configuration: nil,
		Metadata:      nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	i := 0
	for {
		result, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		b, err := json.Marshal(result)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("chunk[%d]: %s\n", i, string(b))
		i++
	}

	// error
	stream, err = cli.SendMessageStreaming(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("trigger error")},
			},
		},
		Configuration: nil,
		Metadata:      nil,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n\nerror stream chat:")
	i = 0
	for {
		result, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(err)
			return
		}
		b, err := json.Marshal(result)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("chunk[%d]: %s\n", i, string(b))
		i++
	}
}

func resubscribeChat(ctx context.Context, cli *client.A2AClient) {
	stream, err := cli.SendMessageStreaming(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("hello")},
			},
		},
		Configuration: nil,
		Metadata:      nil,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("first:")
	var taskID string
	i := 0
	for {
		if i == 2 {
			err = stream.Close()
			if err != nil {
				log.Fatal(err)
			}
			break
		}
		result, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		taskID = result.Task.ID

		b, err := json.Marshal(result)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("chunk[%d]: %s\n", i, string(b))
		i++
	}

	stream, err = cli.ResubscribeTask(ctx, &models.TaskIDParams{
		ID: taskID,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("resubscribe:")
	for {
		result, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		b, err := json.Marshal(result)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("chunk[%d]: %s\n", i, string(b))
		i++
	}
}

func notifyChat(ctx context.Context, cli *client.A2AClient) {
	wg := &sync.WaitGroup{}
	// start local server to receive
	wg.Add(1)
	go func() {
		startNotificationServer(wg)
	}()

	_, err := cli.SendMessageStreaming(ctx, &models.MessageSendParams{
		Message: models.Message{
			Role: models.RoleUser,
			Parts: []models.Part{
				{Kind: models.PartKindText, Text: ptrOf("hello")},
			},
		},
		Configuration: &models.MessageSendConfiguration{PushNotificationConfig: &models.PushNotificationConfig{
			URL:            "http://localhost:12345/test",
			Token:          "",
			Authentication: nil,
		}},
		Metadata: nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Second * 10)
	wg.Done()
}

func printTask(t *models.Task) string {
	b, err := json.MarshalIndent(t, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
}

func ptrOf[T any](v T) *T {
	return &v
}

func startNotificationServer(wg *sync.WaitGroup) {
	// 注册处理函数到 /test 路径
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 读取整个请求体
		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}

		safeBody := strings.ReplaceAll(string(body), "\n", "")
		safeBody = strings.ReplaceAll(safeBody, "\r", "")
		fmt.Printf("Received POST request body:\n%s\n", safeBody)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request body received"))
	})

	fmt.Println("Server starting on :12345...")
	go func() {
		_ = http.ListenAndServe(":12345", nil)
	}()
	wg.Wait()
}
