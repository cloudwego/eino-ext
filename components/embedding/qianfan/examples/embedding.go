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
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/qianfan"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	qcfg.AccessKey = os.Getenv("QIANFAN_ACCESS_KEY")
	qcfg.SecretKey = os.Getenv("QIANFAN_SECRET_KEY")

	emb, err := qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
		Model: "Embedding-V1",
	})
	if err != nil {
		log.Fatalf("NewEmbedder of qianfan embedding failed, err=%v", err)
	}

	v, err := emb.EmbedStrings(ctx, []string{"hello world", "bye world"})
	if err != nil {
		log.Fatalf("EmbedStrings of qianfan embedding failed, err=%v", err)
	}

	b, _ := json.Marshal(v)
	fmt.Println(string(b))
	// [
	//    [
	//        0.08621871471405029,
	//        -0.0012516016140580177,
	//        -0.09416878968477249,
	//        0.11720088124275208,
	//        ...
	//    ],
	//    [
	//        0.09814976155757904,
	//        0.10714524984359741,
	//        0.06678730994462967,
	//        0.08447521179914474,
	//        ...
	//    ]
	//]
}
