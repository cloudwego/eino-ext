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
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/tencentcloud"
)

func main() {
	ctx := context.Background()

	region := "ap-guangzhou"
	secretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	secretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")

	emb, err := tencentcloud.NewEmbedder(ctx, &tencentcloud.EmbeddingConfig{
		SecretID:  secretID,
		SecretKey: secretKey,
		Region:    region,
	})
	if err != nil {
		panic(err)
	}

	v, err := emb.EmbedStrings(ctx, []string{"hello world", "bye world"})
	if err != nil {
		panic(err)
	}

	b, _ := json.Marshal(v)
	fmt.Println(string(b))
	// [
	//     [0.0265350341796875, -0.0097198486328125, 0.0140533447265625, -0.02789306640625, ...],
	//     [-0.01267242431640625, 0.0211181640625, -0.0018453598022460938, -0.0186614990234375, ...]
	// ]
}
