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

package searxng

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error { b.closed = true; return nil }

type retryRoundTripper func(*http.Request) (*http.Response, error)

func (f retryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSearchClosesRateLimitedBodies(t *testing.T) {
	for _, recoverOnRetry := range []bool{false, true} {
		name := "exhausted"
		if recoverOnRetry {
			name = "recovered"
		}
		t.Run(name, func(t *testing.T) {
			var bodies []*closeTrackingBody
			client, err := NewClient(&ClientConfig{
				BaseUrl: "http://searxng.test/search", MaxRetries: 1,
				HttpClient: &http.Client{Transport: retryRoundTripper(func(r *http.Request) (*http.Response, error) {
					if len(bodies) > 0 && !bodies[len(bodies)-1].closed {
						t.Error("previous response body still open at retry")
					}
					body := &closeTrackingBody{Reader: strings.NewReader(`{"results":[{"title":"test","url":"https://example.com"}]}`)}
					bodies = append(bodies, body)
					status := http.StatusTooManyRequests
					if recoverOnRetry && len(bodies) == 2 {
						status = http.StatusOK
					}
					return &http.Response{StatusCode: status, Body: body, Header: make(http.Header), Request: r}, nil
				})},
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Search(context.Background(), &SearchRequest{Query: "test"})
			if recoverOnRetry && err != nil {
				t.Fatal(err)
			}
			if !recoverOnRetry && (err == nil || err.Error() != "rate limit reached") {
				t.Fatalf("expected rate-limit error, got %v", err)
			}
			if len(bodies) != 2 {
				t.Fatalf("expected 2 attempts, got %d", len(bodies))
			}
			for i, body := range bodies {
				if !body.closed {
					t.Errorf("response body %d was not closed", i)
				}
			}
		})
	}
}
