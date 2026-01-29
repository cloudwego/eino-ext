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

type ConnInfo interface {
	From() Peer
	To() Peer
}

type connInfo struct {
	from Peer
	to   Peer
}

func (c *connInfo) From() Peer {
	return c.from
}

func (c *connInfo) To() Peer {
	return c.to
}

type PeerType uint32

const (
	PeerTypeAddress PeerType = 1
	PeerTypeURL     PeerType = 2
)

type Peer interface {
	Type() PeerType
	Address() string
}

type peer struct {
	typ     PeerType
	address string
}

func (p *peer) Type() PeerType {
	return p.typ
}

func (p *peer) Address() string {
	return p.address
}

func NewPeer(typ PeerType, address string) Peer {
	return &peer{
		typ:     typ,
		address: address,
	}
}

func NewConnInfo(from Peer, to Peer) ConnInfo {
	return &connInfo{
		from: from,
		to:   to,
	}
}
