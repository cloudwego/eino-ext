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

	loader "github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
)

// you can build request by yourself, so you can add custom header、cookie、proxy、timeout etc.

func main() {
	ctx := context.Background()

	urlLoader, err := loader.NewLoader(ctx, &loader.LoaderConfig{
		RequestBuilder: func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error) {
			u, err := url.Parse(source.URI)
			if err != nil {
				return nil, err
			}

			req := &http.Request{
				Method: "GET",
				URL:    u,
			}
			req.Header.Add("auth-token", "xx-token")
			return req, nil
		},
	})
	if err != nil {
		log.Fatalf("NewLoader failed, err=%v", err)
	}

	docs, err := urlLoader.Load(ctx, document.Source{
		URI: "https://some_private_site.com/some_path/some_file",
	})
	if err != nil {
		log.Fatalf("Load failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("%+v\n", doc)
	}
}
