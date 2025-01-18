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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	"github.com/cloudwego/eino/schema"
)

type Custom struct {
	Key1 any `json:"key_1"`
	Key2 any
	Key3 map[string]any             `json:"key_3"`
	Key4 schema.Document            `json:"key_4"`
	Key5 schema.FormatType          `json:"key_5"`
	Key6 schema.ChatMessagePartType `json:"key_6"`
	Key7 []int32                    `json:"key_7"`
	Key8 []*schema.Message          `json:"key_8"`
	Key9 []any                      `json:"key_9"`
}

func Test_UnmarshalJson(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		jsonStr := `{
	"key_1": {
		"_value": {
			"hello": "world"
		},
		"_eino_go_type": "map[string]**string"
	},
	"Key2": {
		"_value": {
			"id": "id",
			"content": "content",
			"meta_data": {
				"k1": {
					"_value": 1,
					"_eino_go_type": "int64"
				},
				"k2": {
					"_value": 1,
					"_eino_go_type": "*int64"
				}
			}
		},
		"_eino_go_type": "**schema.Document"
	},
	"key_3": {
		"k1": {
			"_value": {
				"id": "1",
				"content": "2",
				"meta_data": {
					"k1": {
						"_value": 1,
						"_eino_go_type": "int64"
					},
					"k2": {
						"_value": 1,
						"_eino_go_type": "*int64"
					}
				}
			},
			"_eino_go_type": "schema.Document"
		},
		"k2": {
			"_value": true,
			"_eino_go_type": "bool"
		},
		"k3": {
			"_value": 1.1,
			"_eino_go_type": "float32"
		}
	},
	"key_4": {
		"id": "id",
		"meta_data": {
			"k1": {
				"_value": 1,
				"_eino_go_type": "int64"
			}
		}
	},
	"key_5": 1,
	"key_6": "image_url",
	"key_7": [1, 2],
	"key_8": [{
		"extra": {
			"k1": {
				"_value": [1, 2],
				"_eino_go_type": "[]int32"
			}
		}
	}],
	"key_9": [{
		"_value": {
			"extra": {
				"k1": {
					"_value": [1, 2],
					"_eino_go_type": "[]int32"
				}
			}
		},
		"_eino_go_type": "*schema.Message"
	}]
}`

		RegisterType(generic.TypeOf[**schema.Document]())
		RegisterType(generic.TypeOf[schema.Document]())
		RegisterType(generic.TypeOf[map[string]**string]())
		RegisterType(generic.TypeOf[*schema.Message]())
		RegisterType(generic.TypeOf[[]int32]())

		typ := generic.TypeOf[Custom]()
		result, err := UnmarshalJson([]byte(jsonStr), typ)
		assert.NoError(t, err)
		ins := result.Interface().(Custom)
		assert.Equal(t, **ins.Key1.(map[string]**string)["hello"], "world")
		assert.Equal(t, (*ins.Key2.(**schema.Document)).ID, "id")
		assert.Equal(t, (*ins.Key2.(**schema.Document)).Content, "content")
		assert.Equal(t, (*ins.Key2.(**schema.Document)).MetaData["k1"].(int64), int64(1))
		assert.Equal(t, *(*ins.Key2.(**schema.Document)).MetaData["k2"].(*int64), int64(1))
		assert.Equal(t, ins.Key3["k1"].(schema.Document).ID, "1")
		assert.Equal(t, ins.Key3["k2"], true)
		_, ok := ins.Key3["k3"].(float32)
		assert.True(t, ok)
		assert.Equal(t, ins.Key4.ID, "id")
		assert.Equal(t, ins.Key4.MetaData["k1"].(int64), int64(1))
		assert.Equal(t, ins.Key5, schema.GoTemplate)
		assert.Equal(t, ins.Key6, schema.ChatMessagePartTypeImageURL)
		assert.Equal(t, ins.Key7, []int32{1, 2})
		assert.Equal(t, ins.Key8, []*schema.Message{{Extra: map[string]any{"k1": []int32{1, 2}}}})
		assert.Equal(t, ins.Key9, []any{&schema.Message{Extra: map[string]any{"k1": []int32{1, 2}}}})
	})

	t.Run("map[string]any", func(t *testing.T) {
		jsonStr := `{
	"key_1": {
		"_value": {
			"extra": {
				"k1": {
					"_value": [1, 2],
					"_eino_go_type": "[]int32"
				}
			}
		},
		"_eino_go_type": "*schema.Message"
	}
}`

		RegisterType(generic.TypeOf[*schema.Message]())
		RegisterType(generic.TypeOf[[]int32]())

		typ := generic.TypeOf[map[string]any]()
		result, err := UnmarshalJson([]byte(jsonStr), typ)
		assert.NoError(t, err)
		ins := result.Interface().(map[string]any)
		assert.Equal(t, ins["key_1"].(*schema.Message).Extra["k1"], []int32{1, 2})
	})

	t.Run("any", func(t *testing.T) {
		jsonStr := `{
	"_value": {
		"extra": {
			"k1": {
				"_value": [1, 2],
				"_eino_go_type": "[]int32"
			}
		}
	},
	"_eino_go_type": "*schema.Message"
}`

		RegisterType(generic.TypeOf[*schema.Message]())
		RegisterType(generic.TypeOf[[]int32]())

		typ := generic.TypeOf[any]()
		result, err := UnmarshalJson([]byte(jsonStr), typ)
		assert.NoError(t, err)
		ins := result.Interface().(*schema.Message)
		assert.Equal(t, ins.Extra["k1"], []int32{1, 2})
	})
}

func Test_GetRegisteredTypeJsonSchema(t *testing.T) {
	schemas := GetRegisteredTypeJsonSchema()
	assert.Greater(t, len(schemas), 0)
}
