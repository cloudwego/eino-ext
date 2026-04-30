module github.com/cloudwego/eino-ext/components/model/atlascloud

go 1.18

replace github.com/cloudwego/eino-ext/components/model/openai => ../openai

require (
	github.com/bytedance/mockey v1.3.0
	github.com/cloudwego/eino v0.7.13
	github.com/cloudwego/eino-ext/components/model/openai v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
)
