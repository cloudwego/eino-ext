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

package rpcinfo

type RPCInfo interface {
	Invocation() Invocation
	To() Instance
}

type rpcInfo struct {
	invocation Invocation
	to         Instance
}

func (ri *rpcInfo) Invocation() Invocation {
	return ri.invocation
}

func (ri *rpcInfo) To() Instance {
	return ri.to
}

func NewRPCInfo(invocation Invocation) RPCInfo {
	return &rpcInfo{
		invocation: invocation,
	}
}

// Invocation contains RPC related metadata
type Invocation interface {
	ID() string
	MethodName() string
}

func NewInvocation(id string, methodName string) Invocation {
	return &invocation{
		id:         id,
		methodName: methodName,
	}
}

type invocation struct {
	id         string
	methodName string
}

func (i *invocation) ID() string {
	return i.id
}

func (i *invocation) MethodName() string {
	return i.methodName
}

type Instance interface {
	Address() string
}
