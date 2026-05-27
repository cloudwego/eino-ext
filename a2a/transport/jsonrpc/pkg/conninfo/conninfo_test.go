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

package conninfo

import "testing"

func TestConnInfo(t *testing.T) {
	from := NewPeer(PeerTypeAddress, "127.0.0.1")
	to := NewPeer(PeerTypeURL, "http://example.com")
	info := NewConnInfo(from, to)
	if info.From().Address() != "127.0.0.1" {
		t.Fatalf("unexpected from address")
	}
	if info.To().Type() != PeerTypeURL {
		t.Fatalf("unexpected to type")
	}
}