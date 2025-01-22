package claude

import (
	"context"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	awsCofig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Config contains the configuration options for the Claude model
type Config struct {
	// BaseURL is the custom API endpoint URL
	// Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
	// Optional. Example: "https://custom-claude-api.example.com"
	BaseURL *string

	// ByBedrock indicates whether to use Bedrock Service
	// Required for Bedrock
	ByBedrock bool

	// APIKey is your Bedrock API key
	// Obtain from: https://console.anthropic.com/account/keys
	// Required for Not Bedrock
	APIKey string

	// AccessKey is your Bedrock API Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Required for Bedrock
	AccessKey string

	// SecretAccessKey is your Bedrock API Secret Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Required for Bedrock
	SecretAccessKey string

	// Region is your Bedrock API region
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Required for Bedrock
	Region string

	// Model specifies which Claude model to use
	// Required
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Range: 1 to model's context length
	// Required. Example: 2000 for a medium-length response
	MaxTokens int

	// Temperature controls randomness in responses
	// Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
	// Optional. Example: float32(0.7)
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: float32(0.95)
	TopP *float32

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: int32(40)
	TopK *int32

	// StopSequences specifies custom stop sequences
	// The model will stop generating when it encounters any of these sequences
	// Optional. Example: []string{"\n\nHuman:", "\n\nAssistant:"}
	StopSequences []string
}

func NewClient(ctx context.Context, config *Config) (cli *anthropic.Client, err error) {
	if !config.ByBedrock {
		if config.BaseURL != nil {
			cli = anthropic.NewClient(option.WithBaseURL(*config.BaseURL), option.WithAPIKey(config.APIKey))
		} else {
			cli = anthropic.NewClient(option.WithAPIKey(config.APIKey))
		}
	} else {
		cli = anthropic.NewClient(bedrock.WithLoadDefaultConfig(ctx,
			awsCofig.WithRegion(config.Region),
			awsCofig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				config.AccessKey,
				config.SecretAccessKey,
				"",
			)),
			awsCofig.WithHTTPClient(nil)),
		)
	}

	return
}
