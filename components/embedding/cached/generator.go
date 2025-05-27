package cached

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/cloudwego/eino/components/embedding"
)

var defaultGenerator = NewHashGenerator(sha256.New())

type Generator interface {
	Generate(text string, opts ...embedding.Option) string
}

type HashGenerator struct {
	hash hash.Hash
}

var _ Generator = (*HashGenerator)(nil)

func NewHashGenerator(h hash.Hash) *HashGenerator {
	return &HashGenerator{hash: h}
}

func (g *HashGenerator) Generate(text string, opts ...embedding.Option) string {
	options := embedding.GetCommonOptions(nil, opts...)
	model := ""
	if options.Model != nil {
		model = *options.Model
	}

	plainText := fmt.Sprintf("%s-%x", text, model)
	return fmt.Sprintf("%x", g.hash.Sum([]byte(plainText)))
}
