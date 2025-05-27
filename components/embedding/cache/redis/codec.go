package redis

import "github.com/bytedance/sonic"

var defaultCodec codec = &sonicCodec{}

type codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type sonicCodec struct{}

func (*sonicCodec) Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

func (*sonicCodec) Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}
