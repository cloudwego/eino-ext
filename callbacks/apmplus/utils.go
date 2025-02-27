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

package apmplus

import (
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func getName(info *callbacks.RunInfo) string {
	if len(info.Name) != 0 {
		return info.Name
	}
	return strings.TrimSpace(info.Type + " " + string(info.Component))
}

func convModelCallbackInput(in []callbacks.CallbackInput) []*model.CallbackInput {
	ret := make([]*model.CallbackInput, len(in))
	for i, c := range in {
		ret[i] = model.ConvCallbackInput(c)
	}
	return ret
}

func extractModelInput(ins []*model.CallbackInput) (config *model.Config, messages []*schema.Message, extra map[string]interface{}, err error) {
	var mas [][]*schema.Message
	for _, in := range ins {
		if in == nil {
			continue
		}
		if len(in.Messages) > 0 {
			mas = append(mas, in.Messages)
		}
		if len(in.Extra) > 0 {
			extra = in.Extra
		}
		if in.Config != nil {
			config = in.Config
		}
	}
	if len(mas) == 0 {
		return config, []*schema.Message{}, extra, nil
	}
	messages, err = concatMessageArray(mas)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("concat messages failed: %v", err)
	}
	return config, messages, extra, nil
}

func convModelCallbackOutput(out []callbacks.CallbackOutput) []*model.CallbackOutput {
	ret := make([]*model.CallbackOutput, len(out))
	for i, c := range out {
		ret[i] = model.ConvCallbackOutput(c)
	}
	return ret
}

func extractModelOutput(outs []*model.CallbackOutput) (usage *model.TokenUsage, messages []*schema.Message, extra map[string]interface{}, config *model.Config, err error) {
	masMap := make(map[schema.RoleType][]*schema.Message)
	for _, out := range outs {
		if out == nil {
			continue
		}
		if out.TokenUsage != nil {
			usage = out.TokenUsage
		}
		if out.Message != nil {
			if _, ok := masMap[out.Message.Role]; !ok {
				masMap[out.Message.Role] = make([]*schema.Message, 0)
			}
			masMap[out.Message.Role] = append(masMap[out.Message.Role], out.Message)
		}
		if out.Extra != nil {
			extra = out.Extra
		}
		if out.Config != nil {
			config = out.Config
		}
	}
	if len(masMap) == 0 {
		return usage, nil, extra, config, nil
	}
	messages = make([]*schema.Message, 0)
	for _, mas := range masMap {
		message, err := schema.ConcatMessages(mas)
		if err != nil {
			log.Printf("concat message failed: %v", err)
		} else {
			messages = append(messages, message)
		}
	}

	return usage, messages, extra, config, nil
}

func concatMessageArray(mas [][]*schema.Message) ([]*schema.Message, error) {
	if len(mas) == 0 {
		return nil, fmt.Errorf("message array is empty")
	}
	arrayLen := len(mas[0])

	ret := make([]*schema.Message, arrayLen)
	slicesToConcat := make([][]*schema.Message, arrayLen)

	for _, ma := range mas {
		if len(ma) != arrayLen {
			return nil, fmt.Errorf("unexpected array length. "+
				"Got %d, expected %d", len(ma), arrayLen)
		}

		for i := 0; i < arrayLen; i++ {
			m := ma[i]
			if m != nil {
				slicesToConcat[i] = append(slicesToConcat[i], m)
			}
		}
	}

	for i, slice := range slicesToConcat {
		if len(slice) == 0 {
			ret[i] = nil
		} else if len(slice) == 1 {
			ret[i] = slice[0]
		} else {
			cm, err := schema.ConcatMessages(slice)
			if err != nil {
				return nil, err
			}

			ret[i] = cm
		}
	}

	return ret, nil
}

func convSchemaMessage(in []*schema.Message) []*model.CallbackInput {
	ret := make([]*model.CallbackInput, len(in))
	for i, c := range in {
		ret[i] = model.ConvCallbackInput(c)
	}
	return ret
}
