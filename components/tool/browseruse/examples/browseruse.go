package main

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	// 设置Chrome选项，禁用无头模式以便看到浏览器窗口
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-extensions", false),
	)

	// 创建一个分配器上下文
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// 创建一个浏览器上下文
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	log.Println("正在启动Chrome浏览器...")

	// 打开第一个标签页 (Google)
	if err := chromedp.Run(ctx, chromedp.Navigate("https://www.google.com"), chromedp.Navigate("https://www.baidu.com")); err != nil {
		log.Fatalf("导航到Google失败: %v", err)
	}

	log.Println("成功导航到Google")

}
