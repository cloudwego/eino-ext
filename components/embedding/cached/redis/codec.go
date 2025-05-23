package redis

import "encoding/json"

type codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type jsonCodec struct{}

var defaultCodec codec = &jsonCodec{}

func (j *jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (j *jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
