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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
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

type pointerTextCodec [1]byte

func (p *pointerTextCodec) MarshalText() ([]byte, error) {
	return []byte{p[0]}, nil
}

func (p *pointerTextCodec) UnmarshalText(text []byte) error {
	if len(text) > 0 {
		p[0] = text[0]
	}
	return nil
}

type marshalOnlyTextCodec [1]byte

func (marshalOnlyTextCodec) MarshalText() ([]byte, error) {
	return []byte("value"), nil
}

type unmarshalOnlyTextCodec [1]byte

func (*unmarshalOnlyTextCodec) UnmarshalText([]byte) error {
	return nil
}

type jsonOverrideTextCodec [1]byte

func (jsonOverrideTextCodec) MarshalText() ([]byte, error) {
	return []byte("text"), nil
}

func (*jsonOverrideTextCodec) UnmarshalText([]byte) error {
	return nil
}

func (jsonOverrideTextCodec) MarshalJSON() ([]byte, error) {
	return []byte(`{"value":"json"}`), nil
}

func (*jsonOverrideTextCodec) UnmarshalJSON([]byte) error {
	return nil
}

func Test_isTextJSONType(t *testing.T) {
	uuidType := reflect.TypeOf(uuid.UUID{})
	assert.True(t, isTextJSONType(uuidType))
	assert.True(t, isTextJSONType(reflect.PointerTo(uuidType)))
	assert.True(t, isTextJSONType(reflect.PointerTo(reflect.PointerTo(uuidType))))

	pointerCodecType := reflect.TypeOf(pointerTextCodec{})
	assert.False(t, isTextJSONType(pointerCodecType))
	assert.True(t, isTextJSONType(reflect.PointerTo(pointerCodecType)))
	encoded, err := json.Marshal(&pointerTextCodec{'x'})
	assert.NoError(t, err)
	assert.JSONEq(t, `"x"`, string(encoded))

	assert.False(t, isTextJSONType(reflect.TypeOf(marshalOnlyTextCodec{})))
	assert.False(t, isTextJSONType(reflect.TypeOf(unmarshalOnlyTextCodec{})))
	assert.False(t, isTextJSONType(reflect.TypeOf(jsonOverrideTextCodec{})))
	assert.False(t, isTextJSONType(reflect.PointerTo(reflect.TypeOf(jsonOverrideTextCodec{}))))
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

	t.Run("text JSON types", func(t *testing.T) {
		const id = "550e8400-e29b-41d4-a716-446655440000"
		expected := uuid.MustParse(id)

		type uuidInput struct {
			ID       uuid.UUID            `json:"customer_id"`
			Optional *uuid.UUID           `json:"optional"`
			IDs      []uuid.UUID          `json:"ids"`
			ByName   map[string]uuid.UUID `json:"by_name"`
		}

		result, err := UnmarshalJson([]byte(`{
			"customer_id":"`+id+`",
			"optional":"`+id+`",
			"ids":["`+id+`"],
			"by_name":{"primary":"`+id+`"}
		}`), reflect.TypeOf(uuidInput{}))
		assert.NoError(t, err)
		input := result.Interface().(uuidInput)
		assert.Equal(t, expected, input.ID)
		assert.Equal(t, expected, *input.Optional)
		assert.Equal(t, []uuid.UUID{expected}, input.IDs)
		assert.Equal(t, expected, input.ByName["primary"])

		var ptr **uuid.UUID
		result, err = UnmarshalJson([]byte(`"`+id+`"`), reflect.TypeOf(ptr))
		assert.NoError(t, err)
		assert.Equal(t, expected, **result.Interface().(**uuid.UUID))

		result, err = UnmarshalJson([]byte(`null`), reflect.TypeOf(ptr))
		assert.NoError(t, err)
		assert.Nil(t, result.Interface().(**uuid.UUID))

		RegisterType(reflect.TypeOf(uuid.UUID{}))
		var registeredSchemaFound bool
		for _, registeredSchema := range GetRegisteredTypeJsonSchema() {
			if registeredSchema.Title == "uuid.UUID" {
				registeredSchemaFound = true
				assert.Equal(t, "string", string(registeredSchema.Type))
				break
			}
		}
		assert.True(t, registeredSchemaFound)

		result, err = UnmarshalJson(
			[]byte(`{"_eino_go_type":"uuid.UUID","_value":"`+id+`"}`),
			reflect.TypeOf((*any)(nil)).Elem(),
		)
		assert.NoError(t, err)
		assert.Equal(t, expected, result.Interface().(uuid.UUID))

		assert.NotPanics(t, func() {
			_, err = UnmarshalJson(
				[]byte(`[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`),
				reflect.TypeOf(uuid.UUID{}),
			)
		})
		assert.Error(t, err)

		_, err = UnmarshalJson([]byte(`"not-a-uuid"`), reflect.TypeOf(uuid.UUID{}))
		assert.Error(t, err)
	})

	t.Run("pointer text JSON type", func(t *testing.T) {
		var codec *pointerTextCodec
		result, err := UnmarshalJson([]byte(`"x"`), reflect.TypeOf(codec))
		assert.NoError(t, err)
		assert.Equal(t, byte('x'), result.Interface().(*pointerTextCodec)[0])
	})
}

func Test_GetRegisteredTypeJsonSchema(t *testing.T) {
	schemas := GetRegisteredTypeJsonSchema()
	assert.Greater(t, len(schemas), 0)
}
