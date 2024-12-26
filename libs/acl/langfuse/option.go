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

import "time"

type options struct {
	threads          int
	timeout          time.Duration
	maxTaskQueueSize int
	flushAt          int
	flushInterval    time.Duration
	sampleRate       float64
	logMessage       string
	maskFunc         func(string) string
	maxRetry         uint64
}

type Option func(*options)

func WithThreads(threads int) Option {
	return func(o *options) {
		o.threads = threads
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

func WithMaxTaskQueueSize(maxTaskQueueSize int) Option {
	return func(o *options) {
		o.maxTaskQueueSize = maxTaskQueueSize
	}
}

func WithFlushInterval(flushInterval time.Duration) Option {
	return func(o *options) {
		o.flushInterval = flushInterval
	}
}

func WithSampleRate(sampleRate float64) Option {
	return func(o *options) {
		o.sampleRate = sampleRate
	}
}

func WithLogMessage(logMessage string) Option {
	return func(o *options) {
		o.logMessage = logMessage
	}
}

func WithMaskFunc(maskFunc func(string) string) Option {
	return func(o *options) {
		o.maskFunc = maskFunc
	}
}

func WithMaxRetry(maxRetry uint64) Option {
	return func(o *options) {
		o.maxRetry = maxRetry
	}
}
