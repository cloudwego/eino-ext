package cached

import "encoding/json"

type Codec interface {
	Encode(value any) ([]byte, error)
	Decode(data []byte, value any) error
}

type JsonCodec struct{}

func (j *JsonCodec) Encode(value any) ([]byte, error) {
	return json.Marshal(value)
}

func (j *JsonCodec) Decode(data []byte, value any) error {
	return json.Unmarshal(data, value)
}
