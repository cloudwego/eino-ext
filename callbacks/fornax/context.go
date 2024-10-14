package fornax

import (
	"context"

	"code.byted.org/flowdevops/fornax_sdk"
	"code.byted.org/flowdevops/fornax_sdk/consts"
	"code.byted.org/flowdevops/fornax_sdk/infra/ctxmeta"
)

func InjectUserID(ctx context.Context, userID string) context.Context {
	return fornax_sdk.InjectUserID(ctx, userID)
}

func InjectDeviceID(ctx context.Context, deviceID string) context.Context {
	return fornax_sdk.InjectDeviceID(ctx, deviceID)
}

func InjectThreadID(ctx context.Context, threadID string) context.Context {
	return fornax_sdk.InjectThreadID(ctx, threadID)
}

func getUserID(ctx context.Context) (ID string, ok bool) {
	return ctxmeta.GetPersistentExtra(ctx, consts.FornaxUserID)
}

func getDeviceID(ctx context.Context) (deviceID string, ok bool) {
	return ctxmeta.GetPersistentExtra(ctx, consts.FornaxDeviceID)
}

func getThreadID(ctx context.Context) (threadID string, ok bool) {
	return ctxmeta.GetPersistentExtra(ctx, consts.FornaxThreadID)
}