/*
 * Copyright 2024 CloudWeGo Authors
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

package googlesearch

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/customsearch/v1"
)

func TestGooGleSearchTool(t *testing.T) {
	const mockSearchQuery = "Pregel 和 DAG 两种图运行模式的区别"
	const mockSearchResult = `
{
    "context": {
      "title": "EinoSearchTool"
    },
    "items": [
      {
        "displayLink": "www.cnblogs.com",
        "formattedUrl": "https://www.cnblogs.com/xueqiuqiu/articles/12955291.html",
        "htmlFormattedUrl": "https://www.cnblogs.com/xueqiuqiu/articles/12955291.html",
        "htmlSnippet": "May 25, 2020 \u003cb\u003e...\u003c/b\u003e \u003cb\u003ePregel\u003c/b\u003e 使用\u003cb\u003e两种\u003c/b\u003e方法来实现容错性: Checkpoint 在Superstep 执行前进行，用来保存当前系统的状态。当某一图分区计算失败但Worker 仍然可用时，\u0026nbsp;...",
        "htmlTitle": "图解图算法\u003cb\u003ePregel\u003c/b\u003e: 模型简介与实战案例- 雪球球- 博客园",
        "kind": "customsearch#result",
        "link": "https://www.cnblogs.com/xueqiuqiu/articles/12955291.html",
        "pagemap": {
          "metatags": [
            {
              "referrer": "never",
              "og:image": "https://io-meter.com/img/pregel/pregel-basic-model.png",
              "viewport": "width=device-width, initial-scale=1.0",
              "og:description": "这篇文章是对之前在\u0026#160;SHLUG\u0026#160;月度分享活动上所作演讲\u0026#160;Pregel in Graphs\u0026#160;的总结。为使分享内容清晰易懂，本人绘制了大量原创示意图，这篇文字版的总结也会尽量以这些图示为主。 除了对 Pregel 算法的简单介绍，本文还附加了一个用户追踪画像的实战"
            }
          ],
          "cse_image": [
            {
              "src": "https://io-meter.com/img/pregel/pregel-basic-model.png"
            }
          ]
        },
        "snippet": "May 25, 2020 ... Pregel 使用两种方法来实现容错性: Checkpoint 在Superstep 执行前进行，用来保存当前系统的状态。当某一图分区计算失败但Worker 仍然可用时， ...",
        "title": "图解图算法Pregel: 模型简介与实战案例- 雪球球- 博客园"
      },
      {
        "displayLink": "fuhailin.github.io",
        "formattedUrl": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/",
        "htmlFormattedUrl": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/",
        "htmlSnippet": "Apr 29, 2019 \u003cb\u003e...\u003c/b\u003e 虽然，类似\u003cb\u003ePregel\u003c/b\u003e等图计算框架也是将结果保存在内存当中，但是，这些框架只能支持一些特定的计算\u003cb\u003e模式\u003c/b\u003e，并没有提供一种通用的数据抽象。RDD就是为了满足这种\u0026nbsp;...",
        "htmlTitle": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版) | 赵大寳",
        "kind": "customsearch#result",
        "link": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/",
        "pagemap": {
          "person": [
            {
              "image": "https://gitee.com/fuhailin/Object-Storage-Service/raw/master/uploads-avatar.jpg",
              "name": "赵大寳",
              "description": "赵大寳個人小站"
            },
            {
              "image": "https://gitee.com/fuhailin/Object-Storage-Service/raw/master/uploads-avatar.jpg",
              "name": "赵大寳",
              "description": "赵大寳個人小站"
            }
          ],
          "organization": [
            {
              "name": "赵大寳"
            }
          ],
          "metatags": [
            {
              "og:image": "https://gitee.com/fuhailin/Object-Storage-Service/raw/master/Spark-Action-Transformation.jpg",
              "theme-color": "#222",
              "og:type": "article",
              "article:published_time": "2019-04-29T06:56:00.000Z",
              "twitter:card": "summary",
              "og:site_name": "赵大寳",
              "og:title": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版)",
              "og:description": "IntroductionRDD（Resilient Distributed Dataset）叫做弹性分布式数据集，在之前的Spark基本概念当中我已经介绍过RDD是Spark中最基本的数据结构，是一个不可变的分布式对象集合。Spark的核心是建立在统一的抽象RDD之上，使得Spark的各个组件可以无缝进行集成，在同一个应用程序中完成大数据计算任务。RDD的设计理念源自AMP实验室发表的论文《Res",
              "article:author": "赵大寳",
              "twitter:image": "https://gitee.com/fuhailin/Object-Storage-Service/raw/master/Spark-Action-Transformation.jpg",
              "article:tag": "Python",
              "article:modified_time": "2021-07-23T06:42:04.641Z",
              "viewport": "width=device-width",
              "og:locale": "en_US",
              "og:url": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/index.html"
            }
          ],
          "webpage": [
            {
              "copyrightholder": "赵大寳",
              "copyrightyear": "2021"
            }
          ],
          "cse_image": [
            {
              "src": "https://gitee.com/fuhailin/Object-Storage-Service/raw/master/Spark-Action-Transformation.jpg"
            }
          ],
          "wpheader": [
            {
              "description": "鶸鸡程序员，新世纪农民工"
            }
          ],
          "thing": [
            {
              "name": "大数据",
              "url": "大数据"
            }
          ],
          "article": [
            {
              "articlebody": "IntroductionRDD（Resilient Distributed Dataset）叫做弹性分布式数据集， 在之前的Spark基本概念当中我已经介绍过RDD是Spark中最基本的数据结构，是一个不可变...",
              "datemodified": "2021-07-23T14:42:04+08:00",
              "name": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版)",
              "datecreated": "2019-04-29T14:56:00+08:00",
              "headline": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版)",
              "datepublished": "2019-04-29T14:56:00+08:00",
              "mainentityofpage": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/"
            }
          ]
        },
        "snippet": "Apr 29, 2019 ... 虽然，类似Pregel等图计算框架也是将结果保存在内存当中，但是，这些框架只能支持一些特定的计算模式，并没有提供一种通用的数据抽象。RDD就是为了满足这种 ...",
        "title": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版) | 赵大寳"
      },
      {
        "displayLink": "www.cnblogs.com",
        "formattedUrl": "https://www.cnblogs.com/linhaifeng/p/15919143.html",
        "htmlFormattedUrl": "https://www.cnblogs.com/linhaifeng/p/15919143.html",
        "htmlSnippet": "Feb 21, 2022 \u003cb\u003e...\u003c/b\u003e 一Spark与hadoop Hadoop有两个核心模块，分布式存储模块HDFS和分布式计算模块Mapreduce Spark 支持多种编程语言，包括Java、Python、R 和Scala，\u0026nbsp;...",
        "htmlTitle": "Spark\u003cb\u003e运行\u003c/b\u003e架构- linhaifeng - 博客园",
        "kind": "customsearch#result",
        "link": "https://www.cnblogs.com/linhaifeng/p/15919143.html",
        "pagemap": {
          "metatags": [
            {
              "referrer": "origin-when-cross-origin",
              "og:image": "https://img2022.cnblogs.com/blog/1036857/202202/1036857-20220221153424091-878075794.png",
              "viewport": "width=device-width, initial-scale=1.0",
              "og:description": "一 Spark与hadoop Hadoop有两个核心模块，分布式存储模块HDFS和分布式计算模块Mapreduce Spark 支持多种编程语言，包括 Java、Python、R 和 Scala，同时 Spark 也支持 Hadoop 的底层存储系统 HDFS，但 Spark 不依赖 Hadoop。"
            }
          ],
          "cse_image": [
            {
              "src": "https://img2022.cnblogs.com/blog/1036857/202202/1036857-20220221153424091-878075794.png"
            }
          ]
        },
        "snippet": "Feb 21, 2022 ... 一Spark与hadoop Hadoop有两个核心模块，分布式存储模块HDFS和分布式计算模块Mapreduce Spark 支持多种编程语言，包括Java、Python、R 和Scala， ...",
        "title": "Spark运行架构- linhaifeng - 博客园"
      }
    ],
    "kind": "customsearch#search",
    "queries": {
      "nextPage": [
        {
          "count": 3,
          "cx": "b7154871356604f48",
          "gl": "zh-CN",
          "inputEncoding": "utf8",
          "outputEncoding": "utf8",
          "safe": "off",
          "searchTerms": "Pregel 和 DAG 两种图运行模式的区别",
          "startIndex": 4,
          "title": "Google Custom Search - Pregel 和 DAG 两种图运行模式的区别",
          "totalResults": "1850"
        }
      ],
      "request": [
        {
          "count": 3,
          "cx": "b7154871356604f48",
          "gl": "zh-CN",
          "inputEncoding": "utf8",
          "outputEncoding": "utf8",
          "safe": "off",
          "searchTerms": "Pregel 和 DAG 两种图运行模式的区别",
          "startIndex": 1,
          "title": "Google Custom Search - Pregel 和 DAG 两种图运行模式的区别",
          "totalResults": "1850"
        }
      ]
    },
    "searchInformation": {
      "formattedSearchTime": "0.42",
      "formattedTotalResults": "1,850",
      "searchTime": 0.42296,
      "totalResults": "1850"
    },
    "url": {
      "template": "https://www.googleapis.com/customsearch/v1?q={searchTerms}\u0026num={count?}\u0026start={startIndex?}\u0026lr={language?}\u0026safe={safe?}\u0026cx={cx?}\u0026sort={sort?}\u0026filter={filter?}\u0026gl={gl?}\u0026cr={cr?}\u0026googlehost={googleHost?}\u0026c2coff={disableCnTwTranslation?}\u0026hq={hq?}\u0026hl={hl?}\u0026siteSearch={siteSearch?}\u0026siteSearchFilter={siteSearchFilter?}\u0026exactTerms={exactTerms?}\u0026excludeTerms={excludeTerms?}\u0026linkSite={linkSite?}\u0026orTerms={orTerms?}\u0026dateRestrict={dateRestrict?}\u0026lowRange={lowRange?}\u0026highRange={highRange?}\u0026searchType={searchType}\u0026fileType={fileType?}\u0026rights={rights?}\u0026imgSize={imgSize?}\u0026imgType={imgType?}\u0026imgColorType={imgColorType?}\u0026imgDominantColor={imgDominantColor?}\u0026alt=json",
      "type": "application/json"
    }
  }
`
	const expectedSchema = `
{
  "properties" : {
    "query" : {
      "description" : "queried string to the search engine",
      "type" : "string"
    },
    "num" : {
      "description" : "number of search results to return, valid values are between 1 and 10, inclusive",
      "type" : "integer"
    },
    "offset" : {
      "description" : "the index of the first result to return.",
      "type" : "integer"
    },
    "lang" : {
      "description" : "sets the user interface language, default english. usually represented by a 2-4 letter code in ISO 639-1. e.g. en, ja, zh-CN",
      "type" : "string"
    }
  },
  "additionalProperties" : false,
  "required" : [ "query" ],
  "type" : "object"
}
`
	const expectedToolOutput = `
{
	"query": "Pregel 和 DAG 两种图运行模式的区别",
	"items": [
	  {
		  "snippet": "May 25, 2020 ... Pregel 使用两种方法来实现容错性: Checkpoint 在Superstep 执行前进行，用来保存当前系统的状态。当某一图分区计算失败但Worker 仍然可用时， ...",
		  "desc": "这篇文章是对之前在&#160;SHLUG&#160;月度分享活动上所作演讲&#160;Pregel in Graphs&#160;的总结。为使分享内容清晰易懂，本人绘制了大量原创示意图，这篇文字版的总结也会尽量以这些图示为主。 除了对 Pregel 算法的简单介绍，本文还附加了一个用户追踪画像的实战",
		  "link": "https://www.cnblogs.com/xueqiuqiu/articles/12955291.html",
		  "title": "图解图算法Pregel: 模型简介与实战案例- 雪球球- 博客园"
	  },
	  {
		  "snippet": "Apr 29, 2019 ... 虽然，类似Pregel等图计算框架也是将结果保存在内存当中，但是，这些框架只能支持一些特定的计算模式，并没有提供一种通用的数据抽象。RDD就是为了满足这种 ...",
		  "desc": "IntroductionRDD（Resilient Distributed Dataset）叫做弹性分布式数据集，在之前的Spark基本概念当中我已经介绍过RDD是Spark中最基本的数据结构，是一个不可变的分布式对象集合。Spark的核心是建立在统一的抽象RDD之上，使得Spark的各个组件可以无缝进行集成，在同一个应用程序中完成大数据计算任务。RDD的设计理念源自AMP实验室发表的论文《Res",
		  "link": "https://fuhailin.github.io/Program-with-RDD-in-PySpark/",
		  "title": "Spark入门笔记—编程操作对象RDD与DataFrame(PySpark版) | 赵大寳"
	  },
	  {
		  "link": "https://www.cnblogs.com/linhaifeng/p/15919143.html",
		  "title": "Spark运行架构- linhaifeng - 博客园",
		  "snippet": "Feb 21, 2022 ... 一Spark与hadoop Hadoop有两个核心模块，分布式存储模块HDFS和分布式计算模块Mapreduce Spark 支持多种编程语言，包括Java、Python、R 和Scala， ...",
		  "desc": "一 Spark与hadoop Hadoop有两个核心模块，分布式存储模块HDFS和分布式计算模块Mapreduce Spark 支持多种编程语言，包括 Java、Python、R 和 Scala，同时 Spark 也支持 Hadoop 的底层存储系统 HDFS，但 Spark 不依赖 Hadoop。"
	  }
	]
}
`
	searchResult := &customsearch.Search{}
	err := sonic.UnmarshalString(mockSearchResult, searchResult)
	assert.NoError(t, err)
	ctx := context.Background()

	defer mockey.Mock((*customsearch.CseListCall).Do).Return(
		searchResult, nil).Build().Patch().UnPatch()

	t.Run("query_success", func(t *testing.T) {
		conf := &Config{
			APIKey:         "{mock_api_key}",
			SearchEngineID: "{mock_search_engine_id}",
			Num:            3,
		}

		st, err := NewTool(ctx, conf)
		assert.NoError(t, err)

		tl, err := st.Info(ctx)
		assert.NoError(t, err)

		js, err := tl.ToJSONSchema()
		assert.NoError(t, err)
		body, err := js.MarshalJSON()
		assert.NoError(t, err)

		assert.JSONEq(t, expectedSchema, string(body))

		gsr := &SearchRequest{
			Query: mockSearchQuery,
			Lang:  "zh-CN",
		}

		gsrBody, err := sonic.MarshalString(gsr)
		assert.NoError(t, err)

		toolOut, err := st.InvokableRun(ctx, gsrBody)
		assert.NoError(t, err)

		assert.JSONEq(t, expectedToolOutput, toolOut)
	})

}
