/*
 * Copyright 2025 CloudWeGo Authors
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

package main

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type customLoaderOptions struct {
	Timeout    time.Duration
	RetryCount int
}

func WithTimeout(timeout time.Duration) document.LoaderOption {
	return document.WrapLoaderImplSpecificOptFn(func(o *customLoaderOptions) {
		o.Timeout = timeout
	})
}

func WithRetryCount(count int) document.LoaderOption {
	return document.WrapLoaderImplSpecificOptFn(func(o *customLoaderOptions) {
		o.RetryCount = count
	})
}

func NewCustomLoader(config *Config) (*CustomLoader, error) {
	return &CustomLoader{
		timeout:    config.DefaultTimeout,
		retryCount: config.DefaultRetryCount,
	}, nil
}

type CustomLoader struct {
	timeout    time.Duration
	retryCount int
}

type Config struct {
	DefaultTimeout    time.Duration
	DefaultRetryCount int
}

func (l *CustomLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*schema.Document, error) {
	// 1. 处理 option
	options := &customLoaderOptions{
		Timeout:    l.timeout,
		RetryCount: l.retryCount,
	}
	options = document.GetLoaderImplSpecificOptions(options, opts...)
	var err error

	// 2. 处理错误，并进行错误回调方法
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// 3. 开始加载前的回调
	ctx = callbacks.OnStart(ctx, &document.LoaderCallbackInput{
		Source: src,
	})

	// 4. 执行加载逻辑
	docs, err := l.doLoad(ctx, src, options)

	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnEnd(ctx, &document.LoaderCallbackOutput{
		Source: src,
		Docs:   docs,
	})

	return docs, nil
}

func (l *CustomLoader) doLoad(ctx context.Context, src document.Source, opts *customLoaderOptions) ([]*schema.Document, error) {
	// 实现文档加载逻辑
	// 1. 加载文档内容
	// 2. 构造 Document 对象，注意可在 MetaData 中保存文档来源等重要信息
	return []*schema.Document{{
		Content: "Hello World",
	}}, nil
}
