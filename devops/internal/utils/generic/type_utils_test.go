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

package generic

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type V1Interface interface {
	GetName() string
}
type v1Interface struct {
}

func (v1Interface) GetName() string {
	return "v1"
}
func Test_ValidateReflectTypeAllowOperate(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var s string
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(s))
		assert.True(t, f)
		var ss *string
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(ss))
		assert.True(t, f)
	})
	t.Run("int", func(t *testing.T) {
		var s int
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(s))
		assert.True(t, f)

		var ss *int
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(ss))
		assert.True(t, f)

	})
	t.Run("bool", func(t *testing.T) {
		var s bool
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(s))
		assert.True(t, f)

		var ss *bool
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(ss))
		assert.True(t, f)

	})
	t.Run("float", func(t *testing.T) {
		var s float64
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(s))
		assert.True(t, f)
		var ss *float64
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(ss))
		assert.True(t, f)
	})

	t.Run("interface", func(t *testing.T) {
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(v1Interface{}))
		assert.False(t, f)

	})

	t.Run("map key=string,value=string/int/float/boo/*string/*int/*float/*bool ", func(t *testing.T) {
		m1 := make(map[string]string)
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(m1))
		assert.True(t, f)
		m2 := make(map[string]int)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m2))
		assert.True(t, f)
		m3 := make(map[string]*int)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m3))
		assert.True(t, f)
		m4 := make(map[string]*string)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m4))
		assert.True(t, f)

	})

	t.Run("slice value=string/int/float/boo/*string/*int/*float/*bool", func(t *testing.T) {
		m1 := make([]string, 0)
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(m1))
		assert.True(t, f)
		m2 := make([]*string, 0)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m2))
		assert.True(t, f)
		m3 := make([]int, 0)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m3))
		assert.True(t, f)
		m4 := make([]*int, 0)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m4))
		assert.True(t, f)

		m5 := make([]V1Interface, 0)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m5))
		assert.False(t, f)

	})

	t.Run("struct or *struct", func(t *testing.T) {
		type mapStructV1 struct {
			Name string
			Age  int
		}
		m1 := mapStructV1{}
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(m1))
		assert.True(t, f)

		m2 := &mapStructV1{}
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m2))
		assert.True(t, f)
		type mapStructV2 struct {
			Name  string
			Age   int
			Extra map[string]any
		}

		m3 := mapStructV2{}
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m3))
		assert.True(t, f)

		type mapStructV3 struct {
			Extra map[string]any
		}
		m4 := mapStructV3{}
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m4))
		assert.False(t, f)

		type mapStructV4 struct {
			V1 struct {
				Name string
			}
			Extra map[string]any
		}
		m5 := mapStructV4{}
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m5))
		assert.True(t, f)

		type mapStructV5 struct {
			V1 struct {
				Extra map[string]any
			}
			Extra map[string]any
		}
		m6 := mapStructV5{}
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m6))
		assert.False(t, f)

	})

	t.Run("map key=string,value= struct or *struct", func(t *testing.T) {
		type V1 struct {
			Name string
		}
		m1 := make(map[string]V1)
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(m1))
		assert.True(t, f)

		type V2 struct {
			Name  string
			Extra map[string]any
		}
		m2 := make(map[string]V2)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m2))
		assert.True(t, f)

		type V3 struct {
			Extra map[string]any
		}
		m3 := make(map[string]V3)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m3))
		assert.False(t, f)

		type V4 struct {
			V41 struct {
				Name string
			}
			Extra map[string]any
		}
		m4 := make(map[string]V4)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m4))
		assert.True(t, f)

		type V5 struct {
			V51 struct {
				Name string
			}
			Extra map[string]any
		}
		m5 := make(map[string]*V5)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m5))
		assert.True(t, f)

	})

	t.Run("slice value= struct or *struct", func(t *testing.T) {
		type V1 struct {
			Name string
		}
		m1 := make(map[string]V1)
		f := ValidateInputReflectTypeSupported(reflect.TypeOf(m1))
		assert.True(t, f)

		type V2 struct {
			Name  string
			Extra map[string]any
		}
		m2 := make(map[string]V2)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m2))
		assert.True(t, f)

		type V3 struct {
			Extra map[string]any
		}
		m3 := make(map[string]V3)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m3))
		assert.False(t, f)

		type V4 struct {
			V41 struct {
				Name string
			}
			Extra map[string]any
		}
		m4 := make(map[string]V4)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m4))
		assert.True(t, f)

		type V5 struct {
			V51 struct {
				Name string
			}
			Extra map[string]any
		}
		m5 := make(map[string]*V5)
		f = ValidateInputReflectTypeSupported(reflect.TypeOf(m5))
		assert.True(t, f)

	})
}

func Test_IsMapType(t *testing.T) {
	t.Run("map[string]string", func(t *testing.T) {
		isMap := IsMapType[string, string](TypeOf[map[string]string]())
		assert.True(t, isMap)
	})

	t.Run("string", func(t *testing.T) {
		isMap := IsMapType[string, string](TypeOf[string]())
		assert.False(t, isMap)
	})

	t.Run("map[string]any", func(t *testing.T) {
		isMap := IsMapType[string, any](TypeOf[map[string]any]())
		assert.True(t, isMap)
		isMap = IsMapType[string, any](TypeOf[map[any]any]())
		assert.False(t, isMap)
		isMap = IsMapType[string, any](TypeOf[map[any]string]())
		assert.False(t, isMap)
		isMap = IsMapType[string, any](TypeOf[map[string]string]())
		assert.False(t, isMap)
	})

	t.Run("map[any]any", func(t *testing.T) {
		isMap := IsMapType[any, any](TypeOf[map[any]any]())
		assert.True(t, isMap)
		isMap = IsMapType[any, any](TypeOf[any]())
		assert.False(t, isMap)
	})
}

func Test_UnsupportedInputKind(t *testing.T) {
	assert.False(t, UnsupportedInputKind(reflect.TypeOf("").Kind()))
}
