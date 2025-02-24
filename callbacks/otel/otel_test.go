package otel

import (
	"context"
	"testing"

	internalmodel "github.com/cloudwego/eino-ext/callbacks/otel/internal/model"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var ctx = context.Background()

func TestBasic(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	callbacks.InitCallbackHandlers([]callbacks.Handler{
		NewOTelHandler(WithTracerProvider(tp)),
	})

	chain := compose.NewChain[string, *schema.Message]().
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
			return "output", nil
		})).
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
			return []*schema.Message{
				schema.SystemMessage("You are a robot."),
				schema.UserMessage("No, you are a robot."),
			}, nil
		})).
		AppendChatModel(
			internalmodel.NewMockModel(t,
				internalmodel.WithGenerateFunc(func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
					assert.Len(t, input, 2)
					return schema.AssistantMessage("Yes, i'am a robot.", []schema.ToolCall{}), nil
				}),
			),
		)

	r, err := chain.Compile(ctx)
	require.NoError(t, err)

	message, err := r.Invoke(ctx, "input")
	require.NoError(t, err)

	assert.Equal(t, "Yes, i'am a robot.", message.Content)
	spans := sr.Ended()
	spew.Dump(spans)
}
