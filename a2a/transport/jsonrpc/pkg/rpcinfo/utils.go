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

import "context"

type rpcinfoKey struct{}

func NewCtxWithRPCInfo(ctx context.Context, ri RPCInfo) context.Context {
	if ri == nil {
		return ctx
	}
	return context.WithValue(ctx, rpcinfoKey{}, ri)
}

func RPCInfoFromCtx(ctx context.Context) RPCInfo {
	return ctx.Value(rpcinfoKey{}).(RPCInfo)
}
