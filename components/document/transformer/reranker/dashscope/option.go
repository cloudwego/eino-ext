package dashscope

import einoDocument "github.com/cloudwego/eino/components/document"

type TransformerOptions struct {
	Query    string // 查询文本
	TopN     *uint8 // 排序返回的文档数量
	Instruct string // 添加自定义排序任务类型说明，部分模型生效，以dashscope文档为准。建议使用英文撰写。
}

// WithQuery query参数
func WithQuery(query string) einoDocument.TransformerOption {
	return einoDocument.WrapTransformerImplSpecificOptFn(func(o *TransformerOptions) {
		o.Query = query
	})
}

// WithTopN 设置排序返回的文档数量，如果指定的top_n值大于输入的候选document数量，返回全部候选文档。如果指定的top_n值小于输入的候选document数量，会丢弃一部分文档
func WithTopN(topN uint8) einoDocument.TransformerOption {
	return einoDocument.WrapTransformerImplSpecificOptFn(func(o *TransformerOptions) {
		o.TopN = &topN
	})
}

func WithInstruct(instruct string) einoDocument.TransformerOption {
	return einoDocument.WrapTransformerImplSpecificOptFn(func(o *TransformerOptions) {})
}
