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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
	"github.com/stretchr/testify/assert"
)

func TestNewBrowserUseTool(t *testing.T) {
	ctx := context.Background()

	defer mockey.Mock(chromedp.NewExecAllocator).Return(ctx, func() {}).Build().UnPatch()
	defer mockey.Mock(chromedp.NewContext).Return(ctx, func() {}).Build().UnPatch()
	defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
	defer mockey.Mock((*Tool).updateElements).Return(nil).Build().UnPatch()
	defer mockey.Mock((*Tool).updateTabsInfo).Return(nil).Build().UnPatch()

	tool, err := NewBrowserUseTool(ctx, &Config{
		ChromeInstancePath: "test path",
		ProxyServer:        "proxy server",
	})
	assert.NoError(t, err)
	assert.NotNil(t, tool)

	// 验证工具信息
	info, err := tool.Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, toolName, info.Name)
	tool.Cleanup()
}

func TestExecute(t *testing.T) {
	tool := &Tool{}
	defer mockey.Mock((*Tool).updateElements).Return(nil).Build().UnPatch()
	defer mockey.Mock((*Tool).updateTabsInfo).Return(nil).Build().UnPatch()

	mockey.PatchConvey("go to url", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		url := "test url"
		result, err := tool.Execute(&Param{Action: ActionGoToURL, URL: &url})
		assert.NoError(t, err)
		assert.Equal(t, "successfully navigated to test url", result.Output)
	})
	mockey.PatchConvey("click element", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		tool.elements = make([]ElementInfo, 5)
		index := 3
		result, err := tool.Execute(&Param{Action: ActionClickElement, Index: &index})
		assert.NoError(t, err)
		assert.Equal(t, "successfully clicked element 3", result.Output)
	})
	mockey.PatchConvey("input text", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		tool.elements = make([]ElementInfo, 5)
		index := 2
		text := "test input"
		result, err := tool.Execute(&Param{Action: ActionInputText, Index: &index, Text: &text})
		assert.NoError(t, err)
		assert.Equal(t, "successfully input text 'test input' to element 2", result.Output)
	})

	mockey.PatchConvey("scroll down", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		amount := 300
		result, err := tool.Execute(&Param{Action: ActionScrollDown, ScrollAmount: &amount})
		assert.NoError(t, err)
		assert.Equal(t, "successfully scrolled scroll_down 300 pixels", result.Output)
	})

	mockey.PatchConvey("scroll up", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		amount := 200
		result, err := tool.Execute(&Param{Action: ActionScrollUp, ScrollAmount: &amount})
		assert.NoError(t, err)
		assert.Equal(t, "successfully scrolled scroll_up 200 pixels", result.Output)
	})

	mockey.PatchConvey("web search", t, func() {
		tool.searchTool = &ddgsearch.DDGS{}
		defer mockey.Mock((*ddgsearch.DDGS).Search).Return(&ddgsearch.SearchResponse{
			Results: []ddgsearch.SearchResult{
				{URL: "https://example.com/search"},
			},
		}, nil).Build().UnPatch()
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		defer mockey.Mock(chromedp.NewContext).Return(context.Background(), func() {}).Build().UnPatch()

		query := "test query"
		result, err := tool.Execute(&Param{Action: ActionWebSearch, Query: &query})
		assert.NoError(t, err)
		assert.Contains(t, result.Output, "successfully search web")
	})

	mockey.PatchConvey("wait", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		seconds := 5
		result, err := tool.Execute(&Param{Action: ActionWait, Seconds: &seconds})
		assert.NoError(t, err)
		assert.Equal(t, "successfully waited for 5 seconds", result.Output)
	})

	mockey.PatchConvey("extract content", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		goal := "extract test data"
		result, err := tool.Execute(&Param{Action: ActionExtractContent, Goal: &goal})
		assert.NoError(t, err)
		assert.Contains(t, result.Output, "extract content")
	})

	mockey.PatchConvey("open tab", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		url := "https://example.com/newtab"
		result, err := tool.Execute(&Param{Action: ActionOpenTab, URL: &url})
		assert.NoError(t, err)
		assert.Equal(t, "successfully opened new tab https://example.com/newtab", result.Output)
	})

	mockey.PatchConvey("switch tab", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		defer mockey.Mock(target.ActivateTarget).Return(nil).Build().UnPatch()
		tool.tabs = []TabInfo{
			{ID: 0, TargetID: "tab1"},
			{ID: 1, TargetID: "tab2"},
		}
		tabID := 1
		result, err := tool.Execute(&Param{Action: ActionSwitchTab, TabID: &tabID})
		assert.NoError(t, err)
		assert.Equal(t, "successfully switched to tab 1", result.Output)
	})

	mockey.PatchConvey("close tab", t, func() {
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()
		defer mockey.Mock(page.Close).Return(nil).Build().UnPatch()
		tool.tabs = []TabInfo{
			{ID: 0, TargetID: "tab1"},
			{ID: 1, TargetID: "tab2"},
		}
		result, err := tool.Execute(&Param{Action: ActionCloseTab})
		assert.NoError(t, err)
		assert.Equal(t, "successfully closed current tab", result.Output)
	})

}

func TestUpdateElements(t *testing.T) {
	ctx := context.Background()
	tool := Tool{}

	mockey.PatchConvey("update elements", t, func() {
		// 模拟 chromedp.Run 执行，返回模拟的节点
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()

		// 创建模拟的 cdp.Node 节点
		mockNodes := []*cdp.Node{
			{
				NodeName: "A",
				Attributes: []string{
					"href", "https://example.com",
				},
			},
			{
				NodeName: "BUTTON",
				Attributes: []string{
					"textContent", "Click Me",
				},
			},
			{
				NodeName: "INPUT",
				Attributes: []string{
					"type", "text",
					"placeholder", "Enter text",
				},
			},
			{
				NodeName: "SELECT",
				Attributes: []string{
					"name", "options",
				},
			},
			{
				NodeName: "TEXTAREA",
				Attributes: []string{
					"placeholder", "Enter long text",
				},
			},
		}

		// 模拟 chromedp.Nodes 函数，设置 nodes 参数
		defer mockey.Mock(chromedp.Nodes).To(func(sel interface{}, nodes *[]*cdp.Node, opts ...chromedp.QueryOption) chromedp.QueryAction {
			*nodes = mockNodes
			return nil
		}).Build().UnPatch()

		// 模拟 Node.AttributeValue 方法
		defer mockey.Mock((*cdp.Node).AttributeValue).To(func(n *cdp.Node, name string) string {
			for i := 0; i < len(n.Attributes); i += 2 {
				if i+1 < len(n.Attributes) && n.Attributes[i] == name {
					return n.Attributes[i+1]
				}
			}
			return ""
		}).Build().UnPatch()

		// 模拟 Node.FullXPath 方法
		defer mockey.Mock((*cdp.Node).FullXPath).Return("/html/body/element").Build().UnPatch()

		defer mockey.Mock(calculateVisible).Return(true, nil).Build().UnPatch()

		// 执行 updateElements 方法
		err := tool.updateElements(ctx)
		assert.NoError(t, err)

		// 验证元素是否被正确更新
		assert.Equal(t, 5, len(tool.elements))

		// 验证第一个元素 (Link)
		assert.Equal(t, 0, tool.elements[0].Index)
		assert.Equal(t, "Link: https://example.com", tool.elements[0].Description)
		assert.Equal(t, "A", tool.elements[0].Type)
		assert.Equal(t, "/html/body/element", tool.elements[0].XPath)

		// 验证第二个元素 (Button)
		assert.Equal(t, 1, tool.elements[1].Index)
		assert.Equal(t, "Button: Click Me", tool.elements[1].Description)
		assert.Equal(t, "BUTTON", tool.elements[1].Type)

		// 验证第三个元素 (Input)
		assert.Equal(t, 2, tool.elements[2].Index)
		assert.Equal(t, "Input(text): Enter text", tool.elements[2].Description)
		assert.Equal(t, "INPUT", tool.elements[2].Type)

		// 验证第四个元素 (Select)
		assert.Equal(t, 3, tool.elements[3].Index)
		assert.Equal(t, "Select Dropdown: options", tool.elements[3].Description)
		assert.Equal(t, "SELECT", tool.elements[3].Type)

		// 验证第五个元素 (TextArea)
		assert.Equal(t, 4, tool.elements[4].Index)
		assert.Equal(t, "TextArea: Enter long text", tool.elements[4].Description)
		assert.Equal(t, "TEXTAREA", tool.elements[4].Type)
	})
}

func TestGetCurrentState(t *testing.T) {
	ctx := context.Background()
	tool := Tool{}

	mockey.PatchConvey("get current state", t, func() {
		// 模拟 chromedp.Run 执行
		defer mockey.Mock(chromedp.Run).Return(nil).Build().UnPatch()

		// 模拟 updateElements 方法
		defer mockey.Mock((*Tool).updateElements).Return(nil).Build().UnPatch()

		// 模拟 updateTabsInfo 方法
		defer mockey.Mock((*Tool).updateTabsInfo).Return(nil).Build().UnPatch()

		// 设置模拟的元素和标签页数据
		tool.elements = []ElementInfo{
			{
				Index:       0,
				Description: "Link: https://example.com",
				Type:        "A",
				XPath:       "/html/body/div/a",
			},
			{
				Index:       1,
				Description: "Button: Submit",
				Type:        "BUTTON",
				XPath:       "/html/body/div/form/button",
			},
		}

		// 设置上下文
		tool.ctx = ctx

		// 执行 GetCurrentState 方法
		state, err := tool.GetCurrentState()
		assert.NoError(t, err)

		// 验证状态信息是否正确
		assert.Equal(t, "", state.URL)
		assert.Equal(t, "", state.Title)

		// 验证滚动信息
		assert.Equal(t, 0, state.ScrollInfo.PixelsAbove)
		assert.Equal(t, 0, state.ScrollInfo.PixelsBelow)
		assert.Equal(t, 0, state.ScrollInfo.TotalHeight)
		assert.Equal(t, 0, state.ViewportHeight)

		// 验证截图
		assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("")), state.Screenshot)
	})
}

func TestUpdateTabs(t *testing.T) {
	ctx := context.Background()
	tool := &Tool{}
	defer mockey.Mock(chromedp.Targets).Return([]*target.Info{
		{
			Type:     "page",
			URL:      "https://example.com",
			Title:    "Example",
			TargetID: "id",
		},
	}, nil).Build().UnPatch()
	err := tool.updateTabsInfo(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []TabInfo{
		{
			ID:       0,
			TargetID: "id",
			Title:    "Example",
			URL:      "https://example.com",
		},
	}, tool.tabs)
}
