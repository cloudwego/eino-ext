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

package claude

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestNewBedrockAWSHTTPClient(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		assert.Nil(t, newBedrockAWSHTTPClient(nil))
	})

	t.Run("custom RoundTripper", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, nil
			}),
			Timeout: 2 * time.Second,
		}
		awsClient := newBedrockAWSHTTPClient(client)
		if !assert.NotNil(t, awsClient) {
			return
		}
		assert.Equal(t, client.Timeout, awsClient.GetTimeout())
	})

	t.Run("standard transport", func(t *testing.T) {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableCompression = true
		transport.MaxIdleConns = 37
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS13}
		client := &http.Client{
			Transport: transport,
			Timeout:   3 * time.Second,
		}

		awsClient := newBedrockAWSHTTPClient(client)
		if !assert.NotNil(t, awsClient) {
			return
		}
		assert.Equal(t, client.Timeout, awsClient.GetTimeout())

		gotTransport := awsClient.GetTransport()
		assert.NotSame(t, transport, gotTransport)
		assert.Equal(t, transport.DisableCompression, gotTransport.DisableCompression)
		assert.Equal(t, transport.MaxIdleConns, gotTransport.MaxIdleConns)
		assert.NotSame(t, transport.TLSClientConfig, gotTransport.TLSClientConfig)
		assert.Equal(t, transport.TLSClientConfig.MinVersion, gotTransport.TLSClientConfig.MinVersion)
	})
}

func TestNewChatModelBedrockHTTPClientWithCABundle(t *testing.T) {
	setBedrockTestEnv(t)
	t.Setenv("AWS_CA_BUNDLE", writeTestCABundle(t))

	client := &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		Timeout:   3 * time.Second,
	}

	var (
		chatModel *ChatModel
		err       error
	)
	assert.NotPanics(t, func() {
		chatModel, err = NewChatModel(context.Background(), newBedrockTestConfig(client))
	})
	assert.NoError(t, err)
	assert.NotNil(t, chatModel)
}

func TestNewChatModelBedrockInvalidCABundleReturnsError(t *testing.T) {
	setBedrockTestEnv(t)
	bundlePath := filepath.Join(t.TempDir(), "invalid-ca-bundle.pem")
	if err := os.WriteFile(bundlePath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write invalid CA bundle: %v", err)
	}
	t.Setenv("AWS_CA_BUNDLE", bundlePath)

	var err error
	assert.NotPanics(t, func() {
		_, err = NewChatModel(context.Background(), newBedrockTestConfig(&http.Client{}))
	})
	assert.ErrorContains(t, err, "load AWS config for Bedrock")
	assert.ErrorContains(t, err, "failed to load custom CA bundle PEM file")
}

func TestBedrockRequestUsesCustomHTTPClientWithCABundle(t *testing.T) {
	setBedrockTestEnv(t)
	t.Setenv("AWS_CA_BUNDLE", writeTestCABundle(t))

	originalDefaultTransport := http.DefaultTransport
	fallbackRequestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		fallbackRequestCount++
		return &http.Response{
			StatusCode: http.StatusTeapot,
			Status:     "418 I'm a teapot",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("unexpected default transport request")),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = originalDefaultTransport
	})

	var (
		requestCount  int
		requestHost   string
		requestPath   string
		authorization string
	)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			requestHost = req.URL.Host
			requestPath = req.URL.Path
			authorization = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"id":"msg_test",
					"type":"message",
					"role":"assistant",
					"content":[{"type":"text","text":"hi"}],
					"model":"test-model",
					"stop_reason":"end_turn",
					"stop_sequence":null,
					"usage":{"input_tokens":1,"output_tokens":1}
				}`)),
				Request: req,
			}, nil
		}),
		Timeout: 3 * time.Second,
	}

	chatModel, err := NewChatModel(context.Background(), newBedrockTestConfig(client))
	if !assert.NoError(t, err) {
		return
	}

	message, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "hi", message.Content)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, "bedrock-runtime.us-east-1.amazonaws.com", requestHost)
	assert.Equal(t, "/model/test-model/invoke", requestPath)
	assert.True(t, strings.HasPrefix(authorization, "AWS4-HMAC-SHA256"))
	assert.Zero(t, fallbackRequestCount)
}

func TestBedrockCustomHTTPClientIsUsedForCredentialsWithoutCABundle(t *testing.T) {
	setBedrockTestEnv(t)
	setBedrockWebIdentityEnv(t)
	t.Setenv("AWS_CA_BUNDLE", "")

	var (
		stsRequestCount      int
		bedrockRequestCount  int
		bedrockAuthorization string
	)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasPrefix(req.URL.Host, "sts."):
				stsRequestCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"text/xml"}},
					Body: io.NopCloser(strings.NewReader(`<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
						<AssumeRoleWithWebIdentityResult>
							<AssumedRoleUser>
								<Arn>arn:aws:sts::123456789012:assumed-role/test-role/test-session</Arn>
								<AssumedRoleId>test-role-id:test-session</AssumedRoleId>
							</AssumedRoleUser>
							<Credentials>
								<AccessKeyId>web-identity-access-key</AccessKeyId>
								<SecretAccessKey>web-identity-secret-key</SecretAccessKey>
								<SessionToken>web-identity-session-token</SessionToken>
								<Expiration>2100-01-01T00:00:00Z</Expiration>
							</Credentials>
						</AssumeRoleWithWebIdentityResult>
						<ResponseMetadata><RequestId>request-id</RequestId></ResponseMetadata>
					</AssumeRoleWithWebIdentityResponse>`)),
					Request: req,
				}, nil
			case req.URL.Path == "/model/test-model/invoke":
				bedrockRequestCount++
				bedrockAuthorization = req.Header.Get("Authorization")
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"id":"msg_test",
						"type":"message",
						"role":"assistant",
						"content":[{"type":"text","text":"credential path ok"}],
						"model":"test-model",
						"stop_reason":"end_turn",
						"stop_sequence":null,
						"usage":{"input_tokens":1,"output_tokens":1}
					}`)),
					Request: req,
				}, nil
			default:
				t.Fatalf("unexpected request through custom HTTP client: %s", req.URL)
				return nil, nil
			}
		}),
		Timeout: 3 * time.Second,
	}

	config := newBedrockTestConfig(client)
	config.AccessKey = ""
	config.SecretAccessKey = ""
	chatModel, err := NewChatModel(context.Background(), config)
	if !assert.NoError(t, err) {
		return
	}

	message, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "credential path ok", message.Content)
	assert.Equal(t, 1, stsRequestCount)
	assert.Equal(t, 1, bedrockRequestCount)
	assert.Contains(t, bedrockAuthorization, "Credential=web-identity-access-key/")
}

func newBedrockTestConfig(client *http.Client) *Config {
	return &Config{
		ByBedrock:       true,
		AccessKey:       "test-access-key",
		SecretAccessKey: "test-secret-key",
		Region:          "us-east-1",
		Model:           "test-model",
		MaxTokens:       8,
		HTTPClient:      client,
	}
}

func setBedrockTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
}

func setBedrockWebIdentityEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_ACCESS_KEY",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SECRET_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"AWS_DEFAULT_PROFILE",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI",
	} {
		t.Setenv(key, "")
	}

	configPath := filepath.Join(t.TempDir(), "config")
	credentialsPath := filepath.Join(t.TempDir(), "credentials")
	for _, path := range []string{configPath, credentialsPath} {
		if err := os.WriteFile(path, []byte("[default]\n"), 0o600); err != nil {
			t.Fatalf("write empty AWS config: %v", err)
		}
	}
	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)

	tokenPath := filepath.Join(t.TempDir(), "web-identity-token")
	if err := os.WriteFile(tokenPath, []byte("test-token"), 0o600); err != nil {
		t.Fatalf("write web identity token: %v", err)
	}
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tokenPath)
	t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	t.Setenv("AWS_ROLE_SESSION_NAME", "test-session")
}

func writeTestCABundle(t *testing.T) string {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificate := server.Certificate()
	server.Close()

	bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	path := filepath.Join(t.TempDir(), "ca-bundle.pem")
	if err := os.WriteFile(path, bundle, 0o600); err != nil {
		t.Fatalf("write CA bundle: %v", err)
	}
	return path
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
