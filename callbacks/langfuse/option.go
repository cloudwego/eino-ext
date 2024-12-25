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

package langfuse

type options struct {
	name      string
	userID    string
	sessionID string
	release   string
	tags      []string
	public    bool
}

type Option func(o *options)

// WithName sets the name for the trace
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithUserID sets the user ID for the trace
func WithUserID(userID string) Option {
	return func(o *options) {
		o.userID = userID
	}
}

// WithSessionID sets the session ID for the trace
func WithSessionID(sessionID string) Option {
	return func(o *options) {
		o.sessionID = sessionID
	}
}

// WithRelease sets the release version for the trace
func WithRelease(release string) Option {
	return func(o *options) {
		o.release = release
	}
}

// WithTags sets custom tags for the trace
func WithTags(tags []string) Option {
	return func(o *options) {
		o.tags = tags
	}
}

// WithPublic sets whether the trace is publicly accessible
func WithPublic(public bool) Option {
	return func(o *options) {
		o.public = public
	}
}
