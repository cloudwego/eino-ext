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

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	systemCache()
	//toolInfoCache()
	//sessionCache()
}

func systemCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	breakpoint := claude.SetMessageBreakpoint(&schema.Message{
		Role:    schema.System,
		Content: "The film Dongji Rescue, based on a true historical event, tells the story of Chinese fishermen braving turbulent seas to save strangers — a testament to the nation's capacity to create miracles in the face of adversity.\n\nEighty-three years ago, in 1942, the Japanese military seized the Lisbon Maru ship to transport 1,816 British prisoners of war from Hong Kong to Japan. Passing through the waters near Zhoushan, Zhejiang province, it was torpedoed by a US submarine. Fishermen from Zhoushan rescued 384 prisoners of war and hid them from Japanese search parties.\n\nActor Zhu Yilong, in an exclusive interview with China Daily, said he was deeply moved by the humanity shown in such extreme conditions when he accepted his role. \"This historical event proves that in dire circumstances, Chinese people can extend their goodness to protect both themselves and others,\" he said.\n\nLast Friday, a themed event for Dongji Rescue was held in Zhoushan, a city close to where the Lisbon Maru sank. Descendants of the British POWs and the rescuing fishermen gathered to watch the film and share their reflections.\n\nIn the film, the British POWs are aided by a group of fishermen from Dongji Island, whose courage and compassion cut a path through treacherous waves. After the screening, many descendants were visibly moved.\n\n\"I want to express my deepest gratitude to the Chinese people and the descendants of Chinese fishermen. When the film is released in the UK, I will bring my family and friends to watch it. Heroic acts like this deserve to be known worldwide,\" said Denise Wynne, a descendant of a British POW.\n\n\"I felt the profound friendship between the Chinese and British people through the film,\" said Li Hui, a descendant of a rescuer. Many audience members were brought to tears — some by the fishermen's bravery, others by the enduring spirit of \"never abandoning those in peril\".\n\n\"In times of sea peril, rescue is a must,\" said Wu Buwei, another rescuer's descendant, noting that this value has long been a part of Dongji fishermen's traditions.\n\nCoincidentally, on the morning of Aug 5, a real-life rescue unfolded on the shores of Dongfushan in Dongji town. Two tourists from Shanghai fell into the sea and were swept away by powerful waves. Without hesitation, two descendants of Dongji fishermen — Wang Yubin and Yang Xiaoping — leaped in to save them, braving a wind force of 9 to 10 before pulling them to safety.\n\n\"Our forebearers once rescued many British POWs here. As their descendants, we should carry forward their bravery,\" Wang said. Both rescuers later joined the film's cast and crew at the Zhoushan event, sharing the stage with actors who portrayed the fishermen in a symbolic \"reunion across 83 years\".\n\nChinese actor Wu Lei, who plays A Dang in the film, expressed his respect for the two rescuing fishermen on-site. In the film, A Dang saves a British POW named Thomas Newman. Though they share no common language, they manage to connect. In an interview with China Daily, Wu said, \"They connected through a small globe. A Dang understood when Newman pointed out his home, the UK, on the globe.\"\n\nThe film's director Fei Zhenxiang noted that the Eastern Front in World War II is often overlooked internationally, despite China's 14 years of resistance tying down large numbers of Japanese troops. \"Every Chinese person is a hero, and their righteousness should not be forgotten,\" he said.\n\nGuan Hu, codirector, said, \"We hope that through this film, silent kindness can be seen by the world\", marking the 80th anniversary of victory in the Chinese People's War of Resistance Against Japanese Aggression (1931-45) and the World Anti-Fascist War.",
	})

	for i := 0; i < 2; i++ {
		now := time.Now()

		resp, err := cm.Generate(ctx, []*schema.Message{
			{
				Role: schema.System,
				Content: "You are an AI assistant tasked with analyzing literary works. " +
					"Your goal is to provide insightful commentary on themes.",
			},
			breakpoint,
			{
				Role:    schema.User,
				Content: "Analyze the major themes in the content.",
			},
		})
		if err != nil {
			log.Fatalf("Generate failed, err=%v", err)
		}

		log.Printf("time_consume=%f, output: \n%v", time.Now().Sub(now).Seconds(), resp)
	}
}

func toolInfoCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	breakpoint := claude.SetToolInfoBreakpoint(&schema.ToolInfo{
		Name: "get_time",
		Desc: "Get the current time in a given time zone",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"timezone": {
					Required: true,
					Type:     "string",
					Desc:     "The IANA time zone name, e.g. America/Los_Angeles",
				},
			},
		)},
	)

	mockTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
					"unit": {
						Type: "string",
						Enum: []string{"celsius", "fahrenheit"},
						Desc: "The unit of temperature, either celsius or fahrenheit",
					},
				},
			),
		},
		{
			Name: "mock_tool_1",
			Desc: "The film Dongji Rescue, based on a true historical event, tells the story of Chinese fishermen braving turbulent seas to save strangers — a testament to the nation's capacity to create miracles in the face of adversity.\n\nEighty-three years ago, in 1942, the Japanese military seized the Lisbon Maru ship to transport 1,816 British prisoners of war from Hong Kong to Japan. Passing through the waters near Zhoushan, Zhejiang province, it was torpedoed by a US submarine. Fishermen from Zhoushan rescued 384 prisoners of war and hid them from Japanese search parties.\n\nActor Zhu Yilong, in an exclusive interview with China Daily, said he was deeply moved by the humanity shown in such extreme conditions when he accepted his role. \"This historical event proves that in dire circumstances, Chinese people can extend their goodness to protect both themselves and others,\" he said.\n\nLast Friday, a themed event for Dongji Rescue was held in Zhoushan, a city close to where the Lisbon Maru sank. Descendants of the British POWs and the rescuing fishermen gathered to watch the film and share their reflections.\n\nIn the film, the British POWs are aided by a group of fishermen from Dongji Island, whose courage and compassion cut a path through treacherous waves. After the screening, many descendants were visibly moved.\n\n\"I want to express my deepest gratitude to the Chinese people and the descendants of Chinese fishermen. When the film is released in the UK, I will bring my family and friends to watch it. Heroic acts like this deserve to be known worldwide,\" said Denise Wynne, a descendant of a British POW.\n\n\"I felt the profound friendship between the Chinese and British people through the film,\" said Li Hui, a descendant of a rescuer. Many audience members were brought to tears — some by the fishermen's bravery, others by the enduring spirit of \"never abandoning those in peril\".\n\n\"In times of sea peril, rescue is a must,\" said Wu Buwei, another rescuer's descendant, noting that this value has long been a part of Dongji fishermen's traditions.\n\nCoincidentally, on the morning of Aug 5, a real-life rescue unfolded on the shores of Dongfushan in Dongji town. Two tourists from Shanghai fell into the sea and were swept away by powerful waves. Without hesitation, two descendants of Dongji fishermen — Wang Yubin and Yang Xiaoping — leaped in to save them, braving a wind force of 9 to 10 before pulling them to safety.\n\n\"Our forebearers once rescued many British POWs here. As their descendants, we should carry forward their bravery,\" Wang said. Both rescuers later joined the film's cast and crew at the Zhoushan event, sharing the stage with actors who portrayed the fishermen in a symbolic \"reunion across 83 years\".\n\nChinese actor Wu Lei, who plays A Dang in the film, expressed his respect for the two rescuing fishermen on-site. In the film, A Dang saves a British POW named Thomas Newman. Though they share no common language, they manage to connect. In an interview with China Daily, Wu said, \"They connected through a small globe. A Dang understood when Newman pointed out his home, the UK, on the globe.\"\n\nThe film's director Fei Zhenxiang noted that the Eastern Front in World War II is often overlooked internationally, despite China's 14 years of resistance tying down large numbers of Japanese troops. \"Every Chinese person is a hero, and their righteousness should not be forgotten,\" he said.\n\nGuan Hu, codirector, said, \"We hope that through this film, silent kindness can be seen by the world\", marking the 80th anniversary of victory in the Chinese People's War of Resistance Against Japanese Aggression (1931-45) and the World Anti-Fascist War.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "mock parameter",
					},
				},
			),
		},
		{
			Name: "mock_tool_2",
			Desc: "For the administration, the biggest gain was Japan's commitment to investing $550 billion in the United States, particularly with funds to be directed to sectors deemed strategic, ranging from semiconductors and pharmaceuticals to energy infrastructure and critical minerals.The White House highlighted that the deal represents what it called the \"single largest foreign investment commitment ever secured by any country.\" It touted the agreement as a \"strategic realignment of the U.S.-Japan economic relationship\" that will advance the mutual interests of both countries.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "Bessent said it will check the extent of Japan's compliance every quarter and \"if the president is unhappy, then they will boomerang back to the 25 percent tariff rates, both on cars and the rest of their products.\"",
					},
				},
			),
		},
		{
			Name: "mock_tool_3",
			Desc: "Of the amount, up to 100,000 tons is imported for direct consumption. Most of the remainder is used for animal feed and processing into other food products. Any rice imported to Japan beyond the special quota is subject to a tariff of 341 yen ($2.3) per kilogram.\n\n\n In fiscal 2024, the United States shipped roughly 346,000 tons of rice to Japan, compared with 286,000 tons exported by Thailand and 70,000 tons by Australia, under the quota system, according to Japan's farm ministry.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "Currently, Japan imports about 770,000 tons of tariff-free rice a year under the WTO framework, with the United States the No. 1 exporter, accounting for nearly half of the total, followed by Thailand and Australia.",
					},
				},
			),
		},
		breakpoint,
	}

	chatModelWithTools, err := cm.WithTools(mockTools)
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
	}

	for i := 0; i < 2; i++ {
		now := time.Now()

		resp, err := chatModelWithTools.Generate(ctx, []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a tool calling assistant.",
			},
			{
				Role:    schema.User,
				Content: "Mock the get_weather tool input yourself, and then call get_weather",
			},
		})
		if err != nil {
			log.Fatalf("Generate failed, err=%v", err)
		}

		log.Printf("time_consume=%f, output: \n%v", time.Now().Sub(now).Seconds(), resp)
	}
}

func sessionCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	opts := []model.Option{
		claude.WithEnableAutoCache(true),
	}

	mockTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
					"unit": {
						Type: "string",
						Enum: []string{"celsius", "fahrenheit"},
						Desc: "The unit of temperature, either celsius or fahrenheit",
					},
				},
			),
		},
	}

	chatModelWithTools, err := cm.WithTools(mockTools)
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
		return
	}

	input := []*schema.Message{
		schema.SystemMessage("You are a tool calling assistant."),
		schema.UserMessage("What tools can you call?"),
	}

	resp, err := chatModelWithTools.Generate(ctx, input, opts...)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}
	log.Printf("output_1: \n%v\n\n", resp)

	input = append(input, resp, schema.UserMessage("What am I asking last time?"))

	resp, err = chatModelWithTools.Generate(ctx, input, opts...)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	log.Printf("output_2: \n%v\n\n", resp)
}
