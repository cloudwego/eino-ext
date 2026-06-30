package dashscope

import (
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultApiUrl = "/services/rerank/text-rerank/text-rerank"
)

type RerankerConfig struct {
	ModelName string // 模型
	ApiKey    string // apiKey
	BaseUrl   string // 接口基础URL eg: https://dashscope.aliyuncs.com/api/v1
	TopN      *uint8 // 排序默认返回的top文档数量
}

type httpReqParams struct {
	Model      string     `json:"model"`
	Input      input      `json:"input"`
	Parameters parameters `json:"parameters"`
}

type input struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	Instruct  string   `json:"instruct"`
}

type parameters struct {
	ReturnDocuments bool   `json:"return_documents"`
	TopN            *uint8 `json:"top_n"`
}

func (req *httpReqParams) ToMap() (map[string]interface{}, error) {
	b, err := sonic.Marshal(req)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	err = sonic.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (req *httpReqParams) ToJson() (string, error) {
	str, err := sonic.MarshalString(req)
	if err != nil {
		return "", err
	}
	return str, nil
}

type httpResponse struct {
	StatusCode int    `json:"status_code"`
	RequestId  string `json:"request_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Output     output `json:"output"`
	Usage      usage  `json:"usage"`
}

func (resp *httpResponse) ToDocs(src []*schema.Document) []*schema.Document {

	srcMap := map[int]*schema.Document{}
	for index, doc := range src {
		srcMap[index] = doc
	}

	docs := make([]*schema.Document, len(resp.Output.Results))
	for i, result := range resp.Output.Results {
		sourceDoc := srcMap[result.Index]

		docs[i] = sourceDoc
		docs[i].WithScore(result.RelevanceScore)
	}

	return docs
}

type output struct {
	Results []result `json:"results"`
}

type result struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       document
}

type document struct {
	Text string `json:"text"`
}

type usage struct {
	TotalTokens int `json:"total_tokens"`
}
