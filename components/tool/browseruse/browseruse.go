package browseruse

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	toolName        = "browser_use"
	toolDescription = `
Interact with a web browser to perform various actions such as navigation, element interaction, content extraction, and tab management.This tool provides a comprehensive set of browser automation capabilities:

Navigation:
- 'go_to_url': Go to a specific URL in the current tab
- 'go_back': Go back
- 'refresh': Refresh the current page
- 'web_search': Search the query in the current tab, the query should be a search query like humans search in web, concrete and not vague or super long.More the single most important items.

Element Interaction:
- 'click_element': Click an element by index
- 'input_text': Input text into a form element
- 'scroll_down'/'scroll_up': Scroll the page (
with optional pixel amount
)
- 'scroll_to_text': If you dont find something which you want to interact with, scroll to it
- 'send_keys': Send strings of special keys like Escape, Backspace, Insert, PageDown, Delete, Enter, Shortcuts such as 'Control+o', 'Control+Shift+T' are supported as well. This gets used in keyboard.press.
- 'get_dropdown_options': Get all options from a dropdown
- 'select_dropdown_option': Select dropdown option for interactive element index by the text of the option you want to select

Content Extraction:
- 'extract_content': Extract page content to retrieve specific information from the page, e.g.all company names, a specific description, all information about, links with companies in structured format or simply links

Tab Management:
- 'switch_tab': Switch to a specific tab
- 'open_tab': Open a new tab with a URL
- 'close_tab': Close the current tab

Utility:
- 'wait': Wait for a specified number of seconds
`
)

// Config 浏览器配置
type Config struct {
	// Timeout            time.Duration todo
	Headless           bool     `json:"headless"`
	DisableSecurity    bool     `json:"disable_security"`
	ExtraChromiumArgs  []string `json:"extra_chromium_args"`
	ChromeInstancePath string   `json:"chrome_instance_path"`
	ProxyServer        string   `json:"proxy_server"`
	Logf               func(string, ...any)
}

// ToolResult 工具执行结果
type ToolResult struct {
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	Base64Image string `json:"base64_image,omitempty"`
}

// BrowserState 浏览器状态
type BrowserState struct {
	URL                 string     `json:"url"`
	Title               string     `json:"title"`
	Tabs                []TabInfo  `json:"tabs"`
	InteractiveElements string     `json:"interactive_elements"`
	ScrollInfo          ScrollInfo `json:"scroll_info"`
	ViewportHeight      int        `json:"viewport_height"`
	Screenshot          string     `json:"screenshot"`
}

// TabInfo 标签页信息
type TabInfo struct {
	ID       int       `json:"id"`
	TargetID target.ID `json:"target_id"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
}

// ScrollInfo 滚动信息
type ScrollInfo struct {
	PixelsAbove int `json:"pixels_above"`
	PixelsBelow int `json:"pixels_below"`
	TotalHeight int `json:"total_height"`
}

// ElementInfo 元素信息
type ElementInfo struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Type        string `json:"type"`
	XPath       string `json:"xpath"`
}

// Tool 浏览器使用工具
type Tool struct {
	info *schema.ToolInfo

	mu              sync.Mutex
	ctx             context.Context
	allocatorCtx    context.Context
	allocatorCancel context.CancelFunc
	elements        []ElementInfo
	currentTabID    int
	tabs            []TabInfo
	// timeout         time.Duration todo
}

func (b *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return b.info, nil
}

func (b *Tool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	param := &Param{}
	err := sonic.UnmarshalString(argumentsInJSON, param)
	result, err := b.Execute(param)
	if err != nil {
		return "", err
	}
	content, err := sonic.MarshalString(result)
	if err != nil {
		return "", err
	}
	return content, nil
}

// NewBrowserUseTool 创建新的浏览器工具
func NewBrowserUseTool(ctx context.Context, config *Config) (*Tool, error) {
	if config == nil {
		config = &Config{}
	}
	//timeout := config.Timeout
	//if timeout == 0 {
	//	timeout = time.Second * 30
	//}
	but := &Tool{
		info: &schema.ToolInfo{
			Name: toolName,
			Desc: toolDescription,
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
				Type: openapi3.TypeObject,
				Properties: map[string]*openapi3.SchemaRef{
					"action": {
						Value: &openapi3.Schema{
							Type: openapi3.TypeObject,
							Enum: []interface{}{
								string(ActionGoToURL),
								string(ActionClickElement),
								string(ActionInputText),
								string(ActionScrollDown),
								string(ActionScrollUp),
								string(ActionScrollToText),
								string(ActionSendKeys),
								string(ActionGetDropdownOptions),
								string(ActionSelectDropdownOption),
								string(ActionGoBack),
								string(ActionWebSearch),
								string(ActionWait),
								string(ActionExtractContent),
								string(ActionSwitchTab),
								string(ActionOpenTab),
								string(ActionCloseTab),
							},
							Description: "The browser action to perform",
						},
					},
					"url": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeString,
							Description: "URL for 'go_to_url' or 'open_tab' actions",
						},
					},
					"index": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeInteger,
							Description: "Element index for 'click_element', 'input_text', 'get_dropdown_options', or 'select_dropdown_option' actions",
						},
					},
					"text": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeString,
							Description: "Text for 'input_text', 'scroll_to_text', or 'select_dropdown_option' actions",
						},
					},
					"scroll_amount": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeInteger,
							Description: "Pixels to scroll (positive for down, negative for up) for 'scroll_down' or 'scroll_up' actions",
						},
					},
					"tab_id": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeInteger,
							Description: "Tab ID for 'switch_tab' action",
						},
					},
					"query": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeString,
							Description: "Search query for 'web_search' action",
						},
					},
					//"goal": {
					//	Value: &openapi3.Schema{
					//		Type:        openapi3.TypeString,
					//		Description: "Extraction goal for 'extract_content' action",
					//	},
					//},
					"keys": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeString,
							Description: "Keys to send for 'send_keys' action",
						},
					},
					"seconds": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeInteger,
							Description: "Seconds to wait for 'wait' action",
						},
					},
				},
				Required: []string{},
			}),
		},
		tabs: make([]TabInfo, 0),
		//timeout: timeout,
	}

	err := but.initialize(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize browser: %w", err)
	}
	return but, nil
}

func (b *Tool) initialize(ctx context.Context, config *Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if config == nil {
		return fmt.Errorf("config is required")
	}

	if b.ctx != nil {
		b.Cleanup()
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	}

	if !config.Headless {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts, chromedp.Headless)
	}

	if config.DisableSecurity {
		opts = append(opts, chromedp.Flag("disable-web-security", true))
		opts = append(opts, chromedp.Flag("allow-running-insecure-content", true))
	}

	for _, arg := range config.ExtraChromiumArgs {
		opts = append(opts, chromedp.Flag(arg, true))
	}

	if config.ChromeInstancePath != "" {
		opts = append(opts, chromedp.ExecPath(config.ChromeInstancePath))
	}

	if config.ProxyServer != "" {
		opts = append(opts, chromedp.ProxyServer(config.ProxyServer))
	}

	b.allocatorCtx, b.allocatorCancel = chromedp.NewExecAllocator(ctx, opts...)

	logf := log.Printf
	if config.Logf != nil {
		logf = config.Logf
	}
	b.ctx, _ = chromedp.NewContext(
		b.allocatorCtx,
		chromedp.WithLogf(logf),
	)

	if err := runWithTimeout(b.ctx /*,b.timeout*/); err != nil {
		return fmt.Errorf("failed to start browser: %v", err)
	}

	if err := b.updateTabsInfo(b.ctx); err != nil {
		return fmt.Errorf("failed to update tab info: %v", err)
	}

	return nil
}

func (b *Tool) updateTabsInfo(ctx context.Context) error {
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		return err
	}

	b.tabs = make([]TabInfo, 0)
	for i, t := range targets {
		if t.Type == "page" {
			b.tabs = append(b.tabs, TabInfo{
				ID:       i,
				TargetID: t.TargetID,
				Title:    t.Title,
				URL:      t.URL,
			})
		}
	}

	return nil
}

type Param struct {
	Action Action `json:"action"`

	URL          *string `json:"url,omitempty"`
	Index        *int    `json:"index,omitempty"`
	Text         *string `json:"text,omitempty"`
	ScrollAmount *int    `json:"scroll_amount,omitempty"`
	TabID        *int    `json:"tab_id,omitempty"`
	Query        *string `json:"query,omitempty"`
	//Goal         *string `json:"goal,omitempty"`
	Keys    *string `json:"keys,omitempty"`
	Seconds *int    `json:"seconds,omitempty"`
}

// Action 定义浏览器操作类型
type Action string

// 浏览器操作类型常量
const (
	ActionGoToURL              Action = "go_to_url"
	ActionClickElement         Action = "click_element"
	ActionInputText            Action = "input_text"
	ActionScrollDown           Action = "scroll_down"
	ActionScrollUp             Action = "scroll_up"
	ActionScrollToText         Action = "scroll_to_text"
	ActionSendKeys             Action = "send_keys"
	ActionGetDropdownOptions   Action = "get_dropdown_options"
	ActionSelectDropdownOption Action = "select_dropdown_option"
	ActionGoBack               Action = "go_back"
	ActionWebSearch            Action = "web_search"
	ActionWait                 Action = "wait"
	ActionExtractContent       Action = "extract_content"
	ActionSwitchTab            Action = "switch_tab"
	ActionOpenTab              Action = "open_tab"
	ActionCloseTab             Action = "close_tab"
)

func runWithTimeout(ctx context.Context /* timeout time.Duration,*/, actions ...chromedp.Action) error {
	//ctx, cancel := context.WithTimeout(ctx, timeout)
	//defer cancel()
	return chromedp.Run(ctx, actions...)
}

func (b *Tool) Execute(params *Param) (*ToolResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var result *ToolResult

	switch params.Action {
	case ActionGoToURL:
		if params.URL == nil {
			return nil, fmt.Errorf("url is required for 'go_to_url' action")
		}
		url := *params.URL

		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to navigate to %s: %v", url, err)}, fmt.Errorf("failed to navigate to %s: %w", url, err)
		}

		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully navigated to %s", url)}

	case ActionClickElement:
		if params.Index == nil {
			return nil, fmt.Errorf("index is required for 'click_element' action")
		}
		index := *params.Index
		if index >= len(b.elements) {
			return &ToolResult{Error: fmt.Sprintf("index %d out of range", index)}, fmt.Errorf("index out of range")
		}

		element := b.elements[index]
		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.WaitVisible(element.XPath, chromedp.BySearch),
			chromedp.Click(element.XPath, chromedp.BySearch),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to click element %d: %v", index, err)}, fmt.Errorf("failed to click element %d: %w", index, err)
		}

		// 等待页面加载
		err = chromedp.Run(b.ctx, chromedp.Sleep(1*time.Second))

		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully clicked element %d", index)}

	case ActionInputText:
		text := *params.Text
		index := *params.Index
		if index < 0 || index >= len(b.elements) {
			return &ToolResult{Error: fmt.Sprintf("index %d out of range", index)}, fmt.Errorf("index out of range")
		}

		element := b.elements[index]
		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.WaitVisible(element.XPath, chromedp.BySearch),
			chromedp.Clear(element.XPath, chromedp.BySearch),
			chromedp.SendKeys(element.XPath, text, chromedp.BySearch),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to input text to element %d: %v", index, err)}, fmt.Errorf("failed to input text to element %d: %w", index, err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully input text '%s' to element %d", text, index)}

	case ActionScrollDown, ActionScrollUp:
		direction := 1
		if params.Action == ActionScrollUp {
			direction = -1
		}

		var amount int
		if params.ScrollAmount == nil {
			amount = 500
		} else {
			amount = *params.ScrollAmount
		}

		script := fmt.Sprintf("window.scrollBy(0, %d);", direction*amount)
		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.Evaluate(script, nil),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to scroll: %v", err)}, err
		}

		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully scrolled %s %d pixels", params.Action, amount)}

	case ActionScrollToText:
		if params.Text == nil {
			return nil, fmt.Errorf("text is required for 'scroll_to_text' action")
		}
		text := *params.Text

		// 使用JavaScript查找并滚动到包含指定文本的元素
		script := fmt.Sprintf(`
			(function() {
				const elements = Array.from(document.querySelectorAll('*'));
				for (const el of elements) {
					if (el.textContent.includes('%s')) {
						el.scrollIntoView({behavior: 'smooth', block: 'center'});
						return true;
					}
				}
				return false;
			})()
		`, text)

		var found bool
		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.Evaluate(script, &found),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to scroll to text: %v", err)}, fmt.Errorf("failed to scroll to text: %w", err)
		}

		if !found {
			return &ToolResult{Error: fmt.Sprintf("element containing text '%s' not found", text)}, fmt.Errorf("element containing text '%s' not found", text)
		}

		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully scrolled to text '%s'", text)}

	case ActionSendKeys:
		if params.Keys == nil {
			return nil, fmt.Errorf("keys is required for 'send_keys' action")
		}
		keys := *params.Keys

		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.SendKeys("body", keys, chromedp.ByQuery),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to send keys '%s': %v", keys, err)}, fmt.Errorf("failed to send keys '%s': %w", keys, err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully sent keys '%s'", keys)}

	case ActionWait:
		var seconds = 3 // 默认等待3秒
		if params.Seconds != nil {
			seconds = *params.Seconds
		}

		err := chromedp.Run(b.ctx,
			chromedp.Sleep(time.Duration(seconds)*time.Second),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to wait for %d seconds: %v", seconds, err)}, fmt.Errorf("failed to wait for %d seconds: %w", seconds, err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully waited for %d seconds", seconds)}

	case ActionExtractContent:
		// notice has simplified extract
		var html string
		err := runWithTimeout(b.ctx, /*,b.timeout*/
			chromedp.Evaluate(`document.documentElement.outerHTML`, &html),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("获取页面内容失败: %v", err)}, fmt.Errorf("获取页面内容失败: %w", err)
		}

		// 这里应该使用LLM或其他方式提取内容，简化版本直接返回
		result = &ToolResult{Output: fmt.Sprintf("提取内容目标: %s\n页面内容长度: %d 字符", html, len(html))}

	case ActionOpenTab:
		if params.URL == nil {
			return nil, fmt.Errorf("url is required for 'open_tab' action")
		}
		url := *params.URL

		// 创建新标签页
		newCtx, _ := chromedp.NewContext(b.ctx)
		if err := runWithTimeout(newCtx, /*b.timeout,*/
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to open new tab: %v", err)}, fmt.Errorf("failed to open new tab: %w", err)
		}
		b.ctx = newCtx

		// 更新标签页信息
		if err := b.updateTabsInfo(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update tab information: %w", err)
		}
		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully opened new tab %s", url)}

	case ActionSwitchTab:
		if params.TabID == nil {
			return nil, fmt.Errorf("tabID is required for 'switch_tab' action")
		}
		tabID := *params.TabID

		if tabID < 0 || tabID >= len(b.tabs) {
			return &ToolResult{Error: fmt.Sprintf("tab ID %d out of range", tabID)}, fmt.Errorf("tab ID %d out of range", tabID)
		}

		targetID := b.tabs[tabID].TargetID

		// 创建新的上下文并切换到目标标签页
		newCtx, _ := chromedp.NewContext(b.ctx, chromedp.WithTargetID(targetID))
		err := runWithTimeout(newCtx /*b.timeout,*/, target.ActivateTarget(targetID))
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to switch tab: %v", err)}, fmt.Errorf("failed to switch tab: %w", err)
		}

		// 更新当前上下文
		b.ctx = newCtx
		b.currentTabID = tabID

		if err := b.updateTabsInfo(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update tab information: %w", err)
		}
		// 更新元素信息
		if err := b.updateElements(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to update elements: %w", err)
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully switched to tab %d", tabID)}

	case ActionCloseTab:
		// 关闭当前标签页
		err := runWithTimeout(b.ctx /*,b.timeout*/, page.Close())

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to close tab: %v", err)}, fmt.Errorf("failed to close tab: %w", err)
		}

		//如果还有其他标签页，切换到第一个标签页
		if len(b.tabs) > 1 {
			// 重新获取标签页列表
			if err := b.updateTabsInfo(b.ctx); err != nil {
				return nil, fmt.Errorf("failed to update tab information: %w", err)
			}

			if len(b.tabs) > 0 {
				// 切换到第一个标签页
				newTargetID := b.tabs[0].TargetID

				// 创建新的上下文并切换到目标标签页
				newCtx, _ := chromedp.NewContext(b.ctx, chromedp.WithTargetID(newTargetID))
				b.ctx = newCtx
				b.currentTabID = b.tabs[0].ID

				// 更新元素信息
				if err := b.updateElements(b.ctx); err != nil {
					return nil, fmt.Errorf("failed to update elements: %w", err)
				}
			}
		}

		result = &ToolResult{Output: "successfully closed current tab"}

	default:
		return &ToolResult{Error: fmt.Sprintf("unknown action: %s", params.Action)}, fmt.Errorf("unknown action: %s", params.Action)
	}

	return result, nil
}

// updateElements 更新可交互元素信息
func (b *Tool) updateElements(ctx context.Context) error {
	var nodes []*cdp.Node
	err := runWithTimeout(ctx, /* b.timeout,*/
		chromedp.Nodes("a, button, input, select, textarea", &nodes, chromedp.ByQueryAll),
	)

	if err != nil {
		return err
	}

	b.elements = make([]ElementInfo, 0, len(nodes))

	for i, node := range nodes {
		// 获取元素描述
		var description string

		switch node.NodeName {
		case "A":
			description = fmt.Sprintf("Link: %s", node.AttributeValue("href"))
		case "BUTTON":
			description = fmt.Sprintf("Button: %s", node.AttributeValue("textContent"))
		case "INPUT":
			inputType := node.AttributeValue("type")
			description = fmt.Sprintf("Input(%s): %s", inputType, node.AttributeValue("placeholder"))
		case "SELECT":
			description = fmt.Sprintf("Select Dropdown: %s", node.AttributeValue("name"))
		case "TEXTAREA":
			description = fmt.Sprintf("TextArea: %s", node.AttributeValue("placeholder"))
		}

		// 构建XPath
		xpath, err := b.getXPath(ctx, node)
		if err != nil {
			xpath = fmt.Sprintf("//%s[%d]", node.NodeName, i+1)
		}

		b.elements = append(b.elements, ElementInfo{
			Index:       i,
			Description: description,
			Type:        node.NodeName,
			XPath:       xpath,
		})
	}

	return nil
}

// getXPath 获取元素的XPath
func (b *Tool) getXPath(ctx context.Context, node *cdp.Node) (string, error) {
	var xpath string
	err := runWithTimeout(ctx, /* b.timeout,*/
		chromedp.Evaluate(fmt.Sprintf(`
			function getXPath(node) {
				if (node.nodeType !== 1) return '';
				if (node.id) return '//*[@id="' + node.id + '"]';
				
				const parts = [];
				while (node && node.nodeType === 1) {
					let idx = 0;
					let sibling = node;
					while (sibling) {
						if (sibling.nodeType === 1 && sibling.nodeName === node.nodeName) idx++;
						sibling = sibling.previousSibling;
					}
					const prefix = node.nodeName.toLowerCase();
					parts.unshift(idx > 1 ? prefix + '[' + idx + ']' : prefix);
					node = node.parentNode;
				}
				return '/' + parts.join('/');
			}
			getXPath(document.querySelector('[data-nodeid="%d"]'));
		`, node.NodeID), &xpath),
	)

	return xpath, err
}

// Cleanup 清理浏览器资源
func (b *Tool) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.allocatorCancel != nil {
		b.allocatorCancel()
		b.allocatorCancel = nil
	}

	b.ctx = nil
	b.allocatorCtx = nil
	b.elements = nil
	b.tabs = nil
}

func (b *Tool) GetCurrentState() (*BrowserState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ctx == nil {
		return nil, fmt.Errorf("browser not initialized")
	}

	// 获取当前URL和标题
	var url, title string
	err := runWithTimeout(b.ctx, /*,b.timeout*/
		chromedp.Location(&url),
		chromedp.Title(&title),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get url info: %w", err)
	}

	// 获取滚动信息
	var scrollHeight, clientHeight, scrollTop int64
	err = runWithTimeout(b.ctx, /*,b.timeout*/
		chromedp.Evaluate(`
			(() => {
				return {
					scrollHeight: document.documentElement.scrollHeight,
					clientHeight: document.documentElement.clientHeight,
					scrollTop: document.documentElement.scrollTop
				};
			})()
		`, &struct {
			ScrollHeight *int64 `json:"scrollHeight"`
			ClientHeight *int64 `json:"clientHeight"`
			ScrollTop    *int64 `json:"scrollTop"`
		}{
			&scrollHeight,
			&clientHeight,
			&scrollTop,
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get scroll info: %w", err)
	}

	// 更新元素信息
	if err := b.updateElements(b.ctx); err != nil {
		return nil, fmt.Errorf("failed to update elements: %w", err)
	}

	// 更新标签页信息
	if err := b.updateTabsInfo(b.ctx); err != nil {
		return nil, fmt.Errorf("failed to update tab information: %w", err)
	}

	// 获取截图
	var buf []byte
	err = chromedp.Run(b.ctx,
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// 构建交互元素字符串
	var interactiveElements string
	for _, elem := range b.elements {
		interactiveElements += fmt.Sprintf("[%d] %s\n", elem.Index, elem.Description)
	}

	// 构建状态信息
	return &BrowserState{
		URL:                 url,
		Title:               title,
		Tabs:                b.tabs,
		InteractiveElements: interactiveElements,
		ScrollInfo: ScrollInfo{
			PixelsAbove: int(scrollTop),
			PixelsBelow: int(scrollHeight - clientHeight - scrollTop),
			TotalHeight: int(scrollHeight),
		},
		ViewportHeight: int(clientHeight),
		Screenshot:     base64.StdEncoding.EncodeToString(buf),
	}, nil
}
