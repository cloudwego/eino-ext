package main

import (
	"context"
	"log"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	hertz_server "github.com/cloudwego/hertz/pkg/app/server"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"

	"github.com/cloudwego/eino-ext/a2a/extension/eino"
	"github.com/cloudwego/eino-ext/a2a/extension/eino/examples/server/subagents"
)

func main() {
	ctx := context.Background()
	a := subagents.NewBookRecommendAgent()

	h := hertz_server.Default()
	r, err := jsonrpc.NewRegistrar(ctx, &jsonrpc.ServerConfig{
		Router:      h,
		HandlerPath: "/test",
	})
	if err != nil {
		log.Fatal(err)
	}
	err = eino.RegisterServerHandlers(ctx, a, &eino.ServerConfig{
		Registrar: r,
		ResumeConvertor: func(ctx context.Context, t *models.Task, input *models.Message, metadata map[string]any) ([]adk.AgentRunOption, error) {
			text := ""
			for _, p := range input.Parts {
				if p.Kind == models.PartKindText && p.Text != nil {
					text += *p.Text
				}
			}
			return []adk.AgentRunOption{adk.WithToolOptions([]tool.Option{subagents.WithNewInput(text)})}, nil
		},
		CheckPointStore: &inMemoryStore{},
	})
	if err != nil {
		log.Fatal(err)
	}

	_ = h.Run()
}

type inMemoryStore struct {
	m sync.Map
}

func (i *inMemoryStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	v, ok := i.m.Load(checkPointID)
	if !ok {
		return nil, false, nil
	}
	return v.([]byte), true, nil
}

func (i *inMemoryStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	i.m.Store(checkPointID, checkPoint)
	return nil
}
