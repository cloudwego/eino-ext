package main

import (
"context"
"log"

    "github.comcom/cloudwego/eino-ext/callbacks/langsmith"
    "github.com/cloudwego/eino/callbacks"
    "github.com/cloudwego/eino/compose"

)

func main() {
// 创建一个 langsmith handler
// 在实际应用中，您应该从环境变量或配置文件中获取 API 密钥
cbh, err := langsmith.NewLangsmithHandler(&langsmith.Config{
APIKey: "你的\_LANGSMITH_API_KEY",
})
if err != nil {
log.Fatal(err)
}

    // 将 langsmith 设置为全局回调处理器
    callbacks.AppendGlobalHandlers(cbh)

    // 使用 context 设置 trace 级别的属性
    ctx := context.Background()
    ctx = langsmith.SetTrace(ctx,
    	langsmith.WithProjectName("我的项目"),
    	langsmith.WithMetadata(map[string]interface{}{"environment": "production"}),
    )

    // 构建并编译一个 eino graph
    g := compose.NewGraph[string, string]()
    // ... 在这里添加你的节点和边
    runner, _ := g.Compile(ctx)

    // 执行 runner
    result, _ := runner.Invoke(ctx, "一些输入")

    // 处理结果
    log.Printf("得到结果: %s", result)

}
