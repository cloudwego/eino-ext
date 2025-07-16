package apmplus

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
)

// Test_SetSession 为SetSession函数编写的测试函数
func Test_SetSession(t *testing.T) {
	mockey.PatchConvey("测试SetSession函数", t, func() {
		mockey.PatchConvey("不传入SessionOption参数", func() {
			// 初始化一个上下文
			ctx := context.Background()
			// 调用待测函数
			newCtx := SetSession(ctx)
			// 从上下文中获取sessionOptions
			options, ok := newCtx.Value(apmplusSessionOptionKey{}).(*sessionOptions)
			// 断言获取操作成功
			convey.So(ok, convey.ShouldBeTrue)
			// 断言sessionOptions的UserID为空字符串
			convey.So(options.UserID, convey.ShouldEqual, "")
			// 断言sessionOptions的SessionID为空字符串
			convey.So(options.SessionID, convey.ShouldEqual, "")
		})

		mockey.PatchConvey("传入一个SessionOption参数", func() {

			// 初始化一个上下文
			ctx := context.Background()
			// 调用待测函数，传入SessionOption参数
			newCtx := SetSession(ctx, WithUserID("testUser"), WithSessionID("testSession"))
			// 从上下文中获取sessionOptions
			options, ok := newCtx.Value(apmplusSessionOptionKey{}).(*sessionOptions)
			// 断言获取操作成功
			convey.So(ok, convey.ShouldBeTrue)
			// 断言sessionOptions的UserID为"testUser"
			convey.So(options.UserID, convey.ShouldEqual, "testUser")
			convey.So(options.SessionID, convey.ShouldEqual, "testSession")

		})
	})
}
