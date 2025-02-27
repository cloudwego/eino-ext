/*
 * Copyright 2024 CloudWeGo Authors
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
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	loader "github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
)

// you can use any proxy in loader, because you can set your own client.

func main() {
	proxyURL := "http://127.0.0.1:1080"
	u, err := url.Parse(proxyURL)
	if err != nil {
		log.Fatalf("parse proxy url failed, err=%v", err)
	}

	ctx := context.Background()
	urlLoader, err := loader.NewLoader(ctx, &loader.LoaderConfig{
		Client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(u),
			},
		},
	})
	if err != nil {
		log.Fatalf("NewLoader of url loader failed, err=%v", err)
	}

	docs, err := urlLoader.Load(ctx, document.Source{
		URI: "https://some_private_site.com",
	})
	if err != nil {
		log.Fatalf("Load of url loader failed, err=%v", err)
	}
	for _, doc := range docs {
		fmt.Printf("%+v\n", doc)
	}
}
