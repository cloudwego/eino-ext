package dashscope

import (
	"context"
	"errors"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/httprequest/post"
	einoDocument "github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type Reranker struct {
	cfg *RerankerConfig
}

func NewReranker(ctx context.Context, cfg *RerankerConfig) *Reranker {
	return &Reranker{
		cfg: cfg,
	}
}

func (r *Reranker) Transform(ctx context.Context, src []*schema.Document, opts ...einoDocument.TransformerOption) ([]*schema.Document, error) {

	// 1. 处理Option
	options := &TransformerOptions{
		TopN: r.cfg.TopN,
	}

	options = einoDocument.GetTransformerImplSpecificOptions(options, opts...)

	// 2. 发起请求
	contents := make([]string, len(src))
	for i, doc := range src {
		contents[i] = doc.String()
	}

	reqParams := httpReqParams{
		Model: r.cfg.ModelName,
		Input: input{
			Query:     options.Query,
			Documents: contents,
			Instruct:  options.Instruct,
		},
		Parameters: parameters{
			ReturnDocuments: false,
			TopN:            options.TopN,
		},
	}

	resp, err := r.request(ctx, &reqParams)
	if err != nil {
		return []*schema.Document{}, err
	}

	// 3. 返回数据
	return resp.ToDocs(src), nil

}

func (r *Reranker) request(ctx context.Context, reqParams *httpReqParams) (resp *httpResponse, err error) {
	reqJson, err := reqParams.ToJson()
	if err != nil {
		return nil, err
	}

	requestUrl := r.cfg.BaseUrl + defaultApiUrl

	tool, err := post.NewTool(ctx, &post.Config{
		Headers: map[string]string{
			"Authorization": "Bearer " + r.cfg.ApiKey,
			"Content-Type":  "application/json",
		},
		HttpClient: &http.Client{},
	})

	req := &post.PostRequest{
		URL:  requestUrl,
		Body: reqJson,
	}

	reqStr, err := sonic.MarshalString(req)
	if err != nil {
		return nil, err
	}

	respStr, err := tool.InvokableRun(ctx, reqStr)
	if err != nil {
		return nil, err
	}

	resp = &httpResponse{}
	err = sonic.UnmarshalString(respStr, resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Code) > 0 {
		return resp, errors.New(resp.Message)
	}

	return resp, nil
}
