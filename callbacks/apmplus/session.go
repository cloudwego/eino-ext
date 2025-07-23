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

package apmplus

import "context"

type sessionOptions struct {
	UserID    string
	SessionID string
}

type apmplusSessionOptionKey struct{}

func SetSession(ctx context.Context, opts ...SessionOption) context.Context {
	options := &sessionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return context.WithValue(ctx, apmplusSessionOptionKey{}, options)
}

type SessionOption func(*sessionOptions)

func WithUserID(userID string) SessionOption {
	return func(o *sessionOptions) {
		o.UserID = userID
	}
}
func WithSessionID(sessionID string) SessionOption {
	return func(o *sessionOptions) {
		o.SessionID = sessionID
	}
}
