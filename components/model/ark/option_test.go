package ark

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {

	opt := model.GetImplSpecificOptions(&arkOptions{
		customHeaders: nil,
	}, WithCustomHeader(map[string]string{"k1": "v1"}))

	assert.Equal(t, map[string]string{"k1": "v1"}, opt.customHeaders)
}
