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

package langfuse

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
)

type langfuseTraceOptionKey struct{}

func SetTrace(ctx context.Context, opts ...TraceOption) context.Context {
	options := &traceOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, langfuseTraceOptionKey{}, options)
}

type TraceOption func(*traceOptions)

func WithID(id string) TraceOption {
	return func(o *traceOptions) {
		o.ID = id
	}
}
func WithName(name string) TraceOption {
	return func(o *traceOptions) {
		o.Name = name
	}
}
func WithUserID(userID string) TraceOption {
	return func(o *traceOptions) {
		o.UserID = userID
	}
}
func WithSessionID(sessionID string) TraceOption {
	return func(o *traceOptions) {
		o.SessionID = sessionID
	}
}
func WithRelease(release string) TraceOption {
	return func(o *traceOptions) {
		o.Release = release
	}
}
func WithTags(tags ...string) TraceOption {
	return func(o *traceOptions) {
		o.Tags = tags
	}
}
func WithPublic(public bool) TraceOption {
	return func(o *traceOptions) {
		o.Public = public
	}
}
func WithMetadata(metadata map[string]string) TraceOption {
	return func(o *traceOptions) {
		o.Metadata = metadata
	}
}
func WithEnvironment(environment string) TraceOption {
	return func(o *traceOptions) {
		o.Environment = environment
	}
}
func WithVersion(version string) TraceOption {
	return func(o *traceOptions) {
		o.Version = version
	}
}

type traceOptions struct {
	ID          string
	Name        string
	UserID      string
	SessionID   string
	Release     string
	Tags        []string
	Public      bool
	Metadata    map[string]string
	Environment string
	Version     string
}

func initState(_ context.Context, cli langfuse.Langfuse, options *traceOptions) (*langfuseState, error) {
	traceID, err := cli.CreateTrace(&langfuse.TraceEventBody{
		BaseEventBody: langfuse.BaseEventBody{
			ID:          options.ID,
			Name:        options.Name,
			MetaData:    options.Metadata,
			Environment: options.Environment,
			Version:     options.Version,
		},
		TimeStamp: time.Now(),
		UserID:    options.UserID,
		SessionID: options.SessionID,
		Release:   options.Release,
		Tags:      options.Tags,
		Public:    options.Public,
	})
	if err != nil {
		return nil, fmt.Errorf("create trace error: %v", err)
	}
	s := &langfuseState{
		traceID: traceID,
	}
	return s, nil
}
