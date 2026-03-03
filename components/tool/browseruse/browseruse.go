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

package browseruse

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	toolName        = "browser_use"
	toolDescription = `
Interact with a web browser to perform various actions such as navigation, element interaction, content extraction, and tab management:
Navigation:
- 'go_to_url': Go to a specific URL in the current tab
- 'web_search': Search the query in the current tab, the query should be a search query like humans search in web.
Element Interaction:
- 'click_element': Click an element by index
- 'input_text': Input text into a form element
- 'scroll_down'/'scroll_up': Scroll the page (with optional pixel amount)
Content Extraction:
- 'extract_content': Extract page content to retrieve specific information from the page, e.g.all company names, a specific description, links with companies in structured format or simply links
Tab Management:
- 'switch_tab': Switch to a specific tab
- 'open_tab': Open a new tab with a URL
- 'close_tab': Close the current tab
Utility:
- 'wait': Wait for a specified number of seconds
`

	extractContentPrompt = `
Your task is to extract the content of the page. You will be given a page and a goal, and you should extract all relevant information around this goal from the page. If the goal is vague, summarize the page. Respond in json format.
Extraction goal: {goal}

Page content:
{page}
`
)

type Config struct {
	Headless           bool     `json:"headless"`
	DisableSecurity    bool     `json:"disable_security"`
	ExtraChromiumArgs  []string `json:"extra_chromium_args"`
	ChromeInstancePath string   `json:"chrome_instance_path"`
	ProxyServer        string   `json:"proxy_server"`

	DDGSearchTool    duckduckgo.Search
	ExtractChatModel model.BaseChatModel

	Logf func(string, ...any)
}

type ToolResult struct {
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	Base64Image string `json:"base64_image,omitempty"`
}

type BrowserState struct {
	URL                 string     `json:"url"`
	Title               string     `json:"title"`
	Tabs                []TabInfo  `json:"tabs"`
	InteractiveElements string     `json:"interactive_elements"`
	ScrollInfo          ScrollInfo `json:"scroll_info"`
	ViewportHeight      int        `json:"viewport_height"`
	Screenshot          string     `json:"screenshot"`
}

type TabInfo struct {
	ID       int       `json:"id"`
	TargetID target.ID `json:"target_id"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
}

type ScrollInfo struct {
	PixelsAbove int `json:"pixels_above"`
	PixelsBelow int `json:"pixels_below"`
	TotalHeight int `json:"total_height"`
}

type ElementInfo struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Type        string `json:"type"`
	XPath       string `json:"xpath"`
}

type Tool struct {
	info *schema.ToolInfo

	mu              sync.Mutex
	ctx             context.Context
	allocatorCtx    context.Context
	allocatorCancel context.CancelFunc
	elements        []ElementInfo
	currentTabID    int
	tabs            []TabInfo
	searchTool      duckduckgo.Search
	cm              model.BaseChatModel
	tpl             prompt.ChatTemplate
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

func NewBrowserUseTool(ctx context.Context, config *Config) (*Tool, error) {
	if config == nil {
		config = &Config{}
	}
	but := &Tool{
		info: &schema.ToolInfo{
			Name: toolName,
			Desc: toolDescription,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
				&jsonschema.Schema{
					Type: string(schema.Object),
					Properties: orderedmap.New[string, *jsonschema.Schema](
						orderedmap.WithInitialData[string, *jsonschema.Schema](
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "action",
								Value: &jsonschema.Schema{
									Type: string(schema.String),
									Enum: []any{
										string(ActionGoToURL),
										string(ActionClickElement),
										string(ActionInputText),
										string(ActionScrollDown),
										string(ActionScrollUp),
										//string(ActionSendKeys),
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
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "url",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "URL for 'go_to_url' or 'open_tab' actions",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "index",
								Value: &jsonschema.Schema{
									Type:        string(schema.Integer),
									Description: "Element index for 'click_element', 'input_text' actions",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "text",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Text for 'input_text' actions",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "scroll_amount",
								Value: &jsonschema.Schema{
									Type:        string(schema.Integer),
									Description: "Pixels to scroll (positive for down, negative for up) for 'scroll_down' or 'scroll_up' actions",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "tab_id",
								Value: &jsonschema.Schema{
									Type:        string(schema.Integer),
									Description: "Tab ID for 'switch_tab' action",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "query",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Search query for 'web_search' action",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "goal",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Extraction goal for 'extract_content' action",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "keys",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Keys to send for 'send_keys' action",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "seconds",
								Value: &jsonschema.Schema{
									Type:        string(schema.Integer),
									Description: "Seconds to wait for 'wait' action",
								},
							},
						),
					),
				},
			),
		},
		tabs:       make([]TabInfo, 0),
		searchTool: config.DDGSearchTool,
		cm:         config.ExtractChatModel,
		tpl:        prompt.FromMessages(schema.FString, schema.UserMessage(extractContentPrompt)),
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

	logf := func(string, ...any) {}
	if config.Logf != nil {
		logf = config.Logf
	}
	b.ctx, _ = chromedp.NewContext(
		b.allocatorCtx,
		chromedp.WithLogf(logf),
	)

	if err := chromedp.Run(b.ctx); err != nil {
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
	Goal         *string `json:"goal,omitempty"`
	Keys         *string `json:"keys,omitempty"`
	Seconds      *int    `json:"seconds,omitempty"`
}

type Action string

const (
	ActionGoToURL      Action = "go_to_url"
	ActionClickElement Action = "click_element"
	ActionInputText    Action = "input_text"
	ActionScrollDown   Action = "scroll_down"
	ActionScrollUp     Action = "scroll_up"
	//ActionSendKeys       Action = "send_keys"
	ActionWebSearch      Action = "web_search"
	ActionWait           Action = "wait"
	ActionExtractContent Action = "extract_content"
	ActionSwitchTab      Action = "switch_tab"
	ActionOpenTab        Action = "open_tab"
	ActionCloseTab       Action = "close_tab"
)

func (b *Tool) Execute(params *Param) (*ToolResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var result *ToolResult

	switch params.Action {
	case ActionGoToURL:
		if params.URL == nil {
			return &ToolResult{Error: "url is required for 'go_to_url' action"}, nil
		}
		url := *params.URL

		err := chromedp.Run(b.ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to navigate to %s: %v", url, err)}, nil
		}

		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully navigated to %s", url)}

	case ActionClickElement:
		if params.Index == nil {
			return &ToolResult{Error: "index is required for 'click_element' action"}, nil
		}
		index := *params.Index
		if index >= len(b.elements) {
			return &ToolResult{Error: fmt.Sprintf("index %d out of range", index)}, nil
		}

		element := b.elements[index]
		err := chromedp.Run(b.ctx,
			chromedp.WaitVisible(element.XPath, chromedp.BySearch),
			chromedp.Click(element.XPath, chromedp.BySearch),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to click element %d: %v", index, err)}, nil
		}

		err = chromedp.Run(b.ctx, chromedp.Sleep(1*time.Second))

		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully clicked element %d", index)}

	case ActionInputText:
		if params.Text == nil {
			return &ToolResult{Error: "text is required for 'input_text' action"}, nil
		}
		if params.Index == nil {
			return &ToolResult{Error: "index is required for 'input_text' action"}, nil
		}
		text := *params.Text
		index := *params.Index
		if index < 0 || index >= len(b.elements) {
			return &ToolResult{Error: "index out of range"}, nil
		}

		element := b.elements[index]
		err := chromedp.Run(b.ctx,
			chromedp.WaitVisible(element.XPath, chromedp.BySearch),
			chromedp.Clear(element.XPath, chromedp.BySearch),
			chromedp.SendKeys(element.XPath, text, chromedp.BySearch),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to input text to element %d: %v", index, err)}, nil
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
		err := chromedp.Run(b.ctx,
			chromedp.Evaluate(script, nil),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to scroll: %v", err)}, nil
		}

		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully scrolled %s %d pixels", params.Action, amount)}

	case ActionWait:
		var seconds = 3
		if params.Seconds != nil {
			seconds = *params.Seconds
		}

		err := chromedp.Run(b.ctx,
			chromedp.Sleep(time.Duration(seconds)*time.Second),
		)

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to wait for %d seconds: %v", seconds, err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully waited for %d seconds", seconds)}

	case ActionWebSearch:
		if b.searchTool == nil {
			return nil, fmt.Errorf("web search fail, no search tool found")
		}
		if params.Query == nil {
			return &ToolResult{Error: "query is required for 'web_search' action"}, nil
		}
		searchResults, err := b.searchTool.TextSearch(b.ctx, &duckduckgo.TextSearchRequest{
			Query: *params.Query,
		})
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to search: %v", err)}, nil
		}
		if len(searchResults.Results) == 0 {
			return &ToolResult{Error: "search result is empty"}, nil
		}
		newCtx, _ := chromedp.NewContext(b.ctx)
		if err := chromedp.Run(newCtx,
			chromedp.Navigate(searchResults.Results[0].URL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to open new tab: %v", err)}, nil
		}
		b.ctx = newCtx

		if err := b.updateTabsInfo(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update tab information: %v", err)}, nil
		}
		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: "successfully search web and opened new tabL: " + searchResults.Results[0].URL}

	case ActionExtractContent:
		if params.Goal == nil {
			return &ToolResult{Error: "goal is required for 'extract_content' action"}, nil
		}

		var html string
		err := chromedp.Run(b.ctx,
			chromedp.Evaluate(`document.documentElement.outerHTML`, &html),
		)
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("extract content fail: %v", err)}, nil
		}

		if b.cm == nil {
			result = &ToolResult{Output: fmt.Sprintf("extract content: %s", html)}
		} else {
			message, err := b.tpl.Format(b.ctx, map[string]interface{}{
				"goal": *params.Goal,
				"page": html,
			})
			if err != nil {
				return &ToolResult{Error: fmt.Sprintf("format extract prompt fail: %v", err)}, nil
			}

			extractResult, err := b.cm.Generate(b.ctx, message)
			if err != nil {
				return &ToolResult{Error: fmt.Sprintf("generate extract content fail: %v", err)}, nil
			}

			result = &ToolResult{Output: fmt.Sprintf("extract content: %s", extractResult)}
		}

	case ActionOpenTab:
		if params.URL == nil {
			return &ToolResult{Error: "url is required for 'open_tab' action"}, nil
		}
		url := *params.URL

		newCtx, _ := chromedp.NewContext(b.ctx)
		if err := chromedp.Run(newCtx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
		); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to open new tab: %v", err)}, nil
		}
		b.ctx = newCtx

		if err := b.updateTabsInfo(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update tab information: %v", err)}, nil
		}
		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully opened new tab %s", url)}

	case ActionSwitchTab:
		if params.TabID == nil {
			return &ToolResult{Error: "tabID is required for 'switch_tab' action"}, nil
		}
		tabID := *params.TabID

		if tabID < 0 || tabID >= len(b.tabs) {
			return &ToolResult{Error: fmt.Sprintf("tab ID %d out of range", tabID)}, nil
		}

		targetID := b.tabs[tabID].TargetID

		newCtx, _ := chromedp.NewContext(b.ctx, chromedp.WithTargetID(targetID))
		err := chromedp.Run(newCtx, target.ActivateTarget(targetID))
		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to switch tab: %v", err)}, nil
		}

		b.ctx = newCtx
		b.currentTabID = tabID

		if err := b.updateTabsInfo(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update tab information: %v", err)}, nil
		}
		if err := b.updateElements(b.ctx); err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
		}

		result = &ToolResult{Output: fmt.Sprintf("successfully switched to tab %d", tabID)}

	case ActionCloseTab:
		err := chromedp.Run(b.ctx, page.Close())

		if err != nil {
			return &ToolResult{Error: fmt.Sprintf("failed to close tab: %v", err)}, nil
		}

		if len(b.tabs) > 1 {
			if err := b.updateTabsInfo(b.ctx); err != nil {
				return &ToolResult{Error: fmt.Sprintf("failed to update tab information: %v", err)}, nil
			}

			if len(b.tabs) > 0 {
				newTargetID := b.tabs[0].TargetID

				newCtx, _ := chromedp.NewContext(b.ctx, chromedp.WithTargetID(newTargetID))
				b.ctx = newCtx
				b.currentTabID = b.tabs[0].ID

				if err := b.updateElements(b.ctx); err != nil {
					return &ToolResult{Error: fmt.Sprintf("failed to update elements: %v", err)}, nil
				}
			}
		}

		result = &ToolResult{Output: "successfully closed current tab"}

	default:
		return &ToolResult{Error: fmt.Sprintf("unknown action: %s", params.Action)}, nil
	}

	return result, nil
}

func (b *Tool) updateElements(ctx context.Context) error {
	var nodes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Nodes("a, button, input, select, textarea", &nodes, chromedp.ByQueryAll),
	)
	if err != nil {
		return err
	}

	b.elements = make([]ElementInfo, 0, len(nodes))

	var visibleNodes []*cdp.Node
	for _, node := range nodes {
		var isVisible bool
		isVisible, err = calculateVisible(ctx, node)
		if err != nil {
			continue
		}

		if isVisible {
			visibleNodes = append(visibleNodes, node)
		}
	}

	for i, node := range visibleNodes {
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

		b.elements = append(b.elements, ElementInfo{
			Index:       i,
			Description: description,
			Type:        node.NodeName,
			XPath:       node.FullXPath(),
		})
	}

	return nil
}

func calculateVisible(ctx context.Context, node *cdp.Node) (bool, error) {
	isVisible := false
	err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(() => {
				const el = document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if (!el) return false;
				
				// 检查元素是否在视口内
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 || rect.height === 0) return false;
				
				// 检查元素是否被CSS隐藏
				const style = window.getComputedStyle(el);
				if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
				
				return true;
			})()
		`, node.FullXPath()), &isVisible))
	return isVisible, err
}

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

	var url, title string
	err := chromedp.Run(b.ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get url info: %w", err)
	}

	var scrollHeight, clientHeight, scrollTop float64
	err = chromedp.Run(b.ctx,
		chromedp.Evaluate(`
			(() => {
				return {
					scrollHeight: document.documentElement.scrollHeight,
					clientHeight: document.documentElement.clientHeight,
					scrollTop: document.documentElement.scrollTop
				};
			})()
		`, &struct {
			ScrollHeight *float64 `json:"scrollHeight"`
			ClientHeight *float64 `json:"clientHeight"`
			ScrollTop    *float64 `json:"scrollTop"`
		}{
			&scrollHeight,
			&clientHeight,
			&scrollTop,
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get scroll info: %w", err)
	}

	if err := b.updateElements(b.ctx); err != nil {
		return nil, fmt.Errorf("failed to update elements: %w", err)
	}

	if err := b.updateTabsInfo(b.ctx); err != nil {
		return nil, fmt.Errorf("failed to update tab information: %w", err)
	}

	var elementsJS string
	for _, elem := range b.elements {
		elementsJS += fmt.Sprintf(`{xpath: "%s", index: %d},`, elem.XPath, elem.Index)
	}
	err = chromedp.Run(b.ctx, chromedp.Evaluate(fmt.Sprintf(`
		(() => {
			// 移除之前可能存在的标记
			const oldMarkers = document.querySelectorAll('.eino-element-marker, .eino-element-border');
			oldMarkers.forEach(marker => marker.remove());
			
			// 使用XPath查找元素并添加标记
			const elements = [%s];
			
			elements.forEach(elem => {
				try {
					// 使用XPath查找元素
					const result = document.evaluate(elem.xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
					const el = result.singleNodeValue;
					if (!el) return;
					
					// 创建序号标记
					const marker = document.createElement('div');
					marker.className = 'eino-element-marker';
					marker.textContent = elem.index;
					marker.style.position = 'absolute';
					marker.style.zIndex = '10000';
					marker.style.backgroundColor = '#f44336';
					marker.style.color = 'white';
					marker.style.padding = '1px 4px';
					marker.style.borderRadius = '2px';
					marker.style.fontSize = '8px';
					marker.style.fontWeight = 'bold';
					marker.style.boxShadow = '0 0 2px rgba(0,0,0,0.3)';
					
					// 获取元素位置
					const rect = el.getBoundingClientRect();
					marker.style.top = (window.scrollY + rect.top - 10) + 'px';
					marker.style.left = (window.scrollX + rect.left - 5) + 'px';
					
					// 创建元素边框
					const border = document.createElement('div');
					border.className = 'eino-element-border';
					border.style.position = 'absolute';
					border.style.zIndex = '9999';
					border.style.border = '2px solid #f44336';
					border.style.borderRadius = '3px';
					border.style.pointerEvents = 'none';
					
					// 设置边框位置和大小
					border.style.top = (window.scrollY + rect.top) + 'px';
					border.style.left = (window.scrollX + rect.left) + 'px';
					border.style.width = rect.width + 'px';
					border.style.height = rect.height + 'px';
					
					document.body.appendChild(marker);
					document.body.appendChild(border);
				} catch (e) {
					console.error('Error adding marker for element:', e);
				}
			});
		})()
	`, elementsJS), nil))

	if err != nil {
		return nil, fmt.Errorf("failed to add element markers: %w", err)
	}

	var buf []byte
	err = chromedp.Run(b.ctx,
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	var interactiveElements string
	for _, elem := range b.elements {
		interactiveElements += fmt.Sprintf("[%d] %s\n", elem.Index, elem.Description)
	}

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
