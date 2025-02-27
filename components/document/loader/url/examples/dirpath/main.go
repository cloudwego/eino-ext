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

	"github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
)

// url loader can load file from dir path, because you can implement the Client

func main() {
	staticDir := "../testdata"
	ctx := context.Background()
	loader, err := url.NewLoader(ctx, &url.LoaderConfig{
		Client: &http.Client{
			Transport: http.NewFileTransport(http.Dir(staticDir)),
		},
	})
	if err != nil {
		log.Fatalf("NewLoader failed, err=%v", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: "file:///test.html",
	})
	if err != nil {
		log.Fatalf("Load failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("%+v\n", doc)
	}
}
