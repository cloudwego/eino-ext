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

package duckduckgo

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func TestClientTextSearch(t *testing.T) {
	PatchConvey("", t, func() {
		mockResults := []*TextSearchResult{
			{URL: "1"},
			{URL: "2"},
			{URL: "3"},
		}

		Mock(buildTextHTMLRequestHeader).Return(http.Header{}).Build()
		Mock((*TextSearchRequest).buildTextHTMLRequestBody).Return(url.Values{}).Build()
		Mock((*client).doTextHTMLSearch).Return(
			Sequence([]*TextSearchResult{{URL: "1"}, {URL: "2"}}, url.Values{"s": []string{"1"}}, nil).
				Then([]*TextSearchResult{{URL: "3"}, {URL: "4"}}, nil, nil)).Build()

		ctx := context.Background()
		cli := &client{
			maxResults: 3,
			httpCli:    &http.Client{},
		}

		resp, err := cli.TextSearch(ctx, &TextSearchRequest{
			Query: "eino",
		})
		assert.NoError(t, err)
		assert.Equal(t, mockResults, resp.Results)
	})
}

func TestParseSearchResponse(t *testing.T) {
	PatchConvey("found results", t, func() {
		respBody := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">

<!--[if IE 6]><html class="ie6" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 7]><html class="lt-ie8 lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 8]><html class="lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if gt IE 8]><!--><html xmlns="http://www.w3.org/1999/xhtml"><!--<![endif]-->
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=3.0, user-scalable=1" />
  <meta name="referrer" content="origin" />
  <meta name="HandheldFriendly" content="true" />
  <meta name="robots" content="noindex, nofollow" />
  <title>eino at DuckDuckGo</title>
  <link title="DuckDuckGo (HTML)" type="application/opensearchdescription+xml" rel="search" href="//duckduckgo.com/opensearch_html_v2.xml" />
  <link href="//duckduckgo.com/favicon.ico" rel="shortcut icon" />
  <link rel="icon" href="//duckduckgo.com/favicon.ico" type="image/x-icon" />
  <link id="icon60" rel="apple-touch-icon" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_60x60.png?v=2"/>
  <link id="icon76" rel="apple-touch-icon" sizes="76x76" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_76x76.png?v=2"/>
  <link id="icon120" rel="apple-touch-icon" sizes="120x120" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_120x120.png?v=2"/>
  <link id="icon152" rel="apple-touch-icon" sizes="152x152" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_152x152.png?v=2"/>
  <link rel="image_src" href="//duckduckgo.com/assets/icons/meta/DDG-icon_256x256.png">
  <link rel="stylesheet" media="handheld, all" href="//duckduckgo.com/dist/h.0d389dd02de7e1030cde.css" type="text/css"/>
</head>

<body class="body--html">
  <a name="top" id="top"></a>

  <form action="/html/" method="post">
    <input type="text" name="state_hidden" id="state_hidden" />
  </form>

  <div>
    <div class="site-wrapper-border"></div>

    <div id="header" class="header cw header--html">
        <a title="DuckDuckGo" href="/html/" class="header__logo-wrap"></a>


    <form name="x" class="header__form" action="/html/" method="post">

      <div class="search search--header">
          <input name="q" autocomplete="off" class="search__input" id="search_form_input_homepage" type="text" value="eino" />
          <input name="b" id="search_button_homepage" class="search__button search__button--html" value="" title="Search" alt="Search" type="submit" />
      </div>


    
    
    
    

    <div class="frm__select">
      <select name="kl">
      
        <option value="" >All Regions</option>
      
        <option value="ar-es" >Argentina</option>
      
        <option value="au-en" >Australia</option>
      
        <option value="at-de" >Austria</option>
      
        <option value="be-fr" >Belgium (fr)</option>
      
        <option value="be-nl" >Belgium (nl)</option>
      
        <option value="br-pt" >Brazil</option>
      
        <option value="bg-bg" >Bulgaria</option>
      
        <option value="ca-en" >Canada (en)</option>
      
        <option value="ca-fr" >Canada (fr)</option>
      
        <option value="ct-ca" >Catalonia</option>
      
        <option value="cl-es" >Chile</option>
      
        <option value="cn-zh" >China</option>
      
        <option value="co-es" >Colombia</option>
      
        <option value="hr-hr" >Croatia</option>
      
        <option value="cz-cs" >Czech Republic</option>
      
        <option value="dk-da" >Denmark</option>
      
        <option value="ee-et" >Estonia</option>
      
        <option value="fi-fi" >Finland</option>
      
        <option value="fr-fr" >France</option>
      
        <option value="de-de" >Germany</option>
      
        <option value="gr-el" >Greece</option>
      
        <option value="hk-tzh" >Hong Kong</option>
      
        <option value="hu-hu" >Hungary</option>
      
        <option value="is-is" >Iceland</option>
      
        <option value="in-en" >India (en)</option>
      
        <option value="id-en" >Indonesia (en)</option>
      
        <option value="ie-en" >Ireland</option>
      
        <option value="il-en" >Israel (en)</option>
      
        <option value="it-it" >Italy</option>
      
        <option value="jp-jp" >Japan</option>
      
        <option value="kr-kr" >Korea</option>
      
        <option value="lv-lv" >Latvia</option>
      
        <option value="lt-lt" >Lithuania</option>
      
        <option value="my-en" >Malaysia (en)</option>
      
        <option value="mx-es" >Mexico</option>
      
        <option value="nl-nl" >Netherlands</option>
      
        <option value="nz-en" >New Zealand</option>
      
        <option value="no-no" >Norway</option>
      
        <option value="pk-en" >Pakistan (en)</option>
      
        <option value="pe-es" >Peru</option>
      
        <option value="ph-en" >Philippines (en)</option>
      
        <option value="pl-pl" >Poland</option>
      
        <option value="pt-pt" >Portugal</option>
      
        <option value="ro-ro" >Romania</option>
      
        <option value="ru-ru" >Russia</option>
      
        <option value="xa-ar" >Saudi Arabia</option>
      
        <option value="sg-en" >Singapore</option>
      
        <option value="sk-sk" >Slovakia</option>
      
        <option value="sl-sl" >Slovenia</option>
      
        <option value="za-en" >South Africa</option>
      
        <option value="es-ca" >Spain (ca)</option>
      
        <option value="es-es" >Spain (es)</option>
      
        <option value="se-sv" >Sweden</option>
      
        <option value="ch-de" >Switzerland (de)</option>
      
        <option value="ch-fr" >Switzerland (fr)</option>
      
        <option value="tw-tzh" >Taiwan</option>
      
        <option value="th-en" >Thailand (en)</option>
      
        <option value="tr-tr" >Turkey</option>
      
        <option value="us-en" >US (English)</option>
      
        <option value="us-es" >US (Spanish)</option>
      
        <option value="ua-uk" >Ukraine</option>
      
        <option value="uk-en" >United Kingdom</option>
      
        <option value="vn-en" >Vietnam (en)</option>
      
      </select>
    </div>

    <div class="frm__select frm__select--last">
      <select class="" name="df">
      
        <option value="" selected>Any Time</option>
      
        <option value="d" >Past Day</option>
      
        <option value="w" >Past Week</option>
      
        <option value="m" >Past Month</option>
      
        <option value="y" >Past Year</option>
      
      </select>
    </div>

    </form>

    </div>





<!-- Web results are present -->

  <div>
  <div class="serp__results">
  <div id="links" class="results">

      



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://github.com/cloudwego/eino">GitHub - cloudwego/eino: The ultimate LLM/AI application development ...</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://github.com/cloudwego/eino">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/github.com.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://github.com/cloudwego/eino">
                  github.com/cloudwego/eino
                  </a>

                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://github.com/cloudwego/eino"><b>Eino</b> is a framework for building LLM applications in Golang, inspired by LangChain and LlamaIndex. It provides component abstractions, orchestration APIs, best practices, tools and examples for LLM development.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://www.cloudwego.io/zh/docs/eino/overview/">Eino: 概述 | CloudWeGo</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://www.cloudwego.io/zh/docs/eino/overview/">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/www.cloudwego.io.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://www.cloudwego.io/zh/docs/eino/overview/">
                  www.cloudwego.io/zh/docs/eino/overview/
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-06-23T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://www.cloudwego.io/zh/docs/eino/overview/"><b>Eino</b>: 概述 简介. Eino[&#x27;aino] (近似音: i know，希望框架能达到 &quot;i know&quot; 的愿景) 旨在提供基于 Golang 语言的终极大模型应用开发框架。 它从开源社区中的诸多优秀 LLM 应用开发框架，如 LangChain 和 LlamaIndex 等获取灵感，同时借鉴前沿研究成果与实际应用，提供了一个强调简洁性、可扩展性、可靠性与 ...</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://ai-bot.cn/eino/">Eino - 字节跳动开源的大模型应用开发框架 | AI工具集</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://ai-bot.cn/eino/">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/ai-bot.cn.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://ai-bot.cn/eino/">
                  ai-bot.cn/eino/
                  </a>

                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://ai-bot.cn/eino/"><b>Eino</b> 是字节跳动开源的大模型应用开发框架，能帮助开发者高效构建基于大模型的 AI 应用。<b>Eino</b>以 Go 语言为基础，具备稳定的内核、灵活的扩展性和完善的工具生态。<b>Eino</b> 的核心是组件化设计，基于定义不同的组件（如 ChatModel、Lambda 等）和编排方式（如 Chain 和 Graph），开发者能灵活地构建复杂的 ...</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://cloudwego.cn/docs/eino/overview/eino_open_source/">Large Language Model Application Development Framework — Eino is Now ...</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://cloudwego.cn/docs/eino/overview/eino_open_source/">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/cloudwego.cn.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://cloudwego.cn/docs/eino/overview/eino_open_source/">
                  cloudwego.cn/docs/eino/overview/eino_open_source/
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-06-13T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://cloudwego.cn/docs/eino/overview/eino_open_source/">Today, after more than six months of internal use and iteration at ByteDance, the Golang-based comprehensive LLM application development framework — <b>Eino</b>, has been officially open-sourced on CloudWeGo! Based on clear &quot;component&quot; definitions, <b>Eino</b> provides powerful process &quot;orchestration&quot; covering the entire development lifecycle, aiming to help developers create the most ...</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://stable-learn.com/en/eino-open-source-announcement/">ByteDance Open Sources Eino: Golang LLM Development Framework</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://stable-learn.com/en/eino-open-source-announcement/">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/stable-learn.com.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://stable-learn.com/en/eino-open-source-announcement/">
                  stable-learn.com/en/eino-open-source-announcement/
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-01-16T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://stable-learn.com/en/eino-open-source-announcement/"><b>Eino</b> is an open-source framework for creating LLM applications in Golang, with stable core, agile extension, and high reliability. It supports various orchestration paradigms, component extensions, and tool ecosystem for LLM development.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://juejin.cn/post/7460126710085091367">万字解读带你跑通 Eino，带你在 2025 打通 Go LLM 应用开发的任督二脉 想不到 2025 年刚 - 掘金</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://juejin.cn/post/7460126710085091367">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/juejin.cn.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://juejin.cn/post/7460126710085091367">
                  juejin.cn/post/7460126710085091367
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-01-16T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://juejin.cn/post/7460126710085091367"><b>Eino</b> 是 CloudWeGo 团队开源的基于 Golang 的 AI 应用开发框架，支持多种 LLM 模型和组件，提供了简洁、可扩展、可靠、有效的开发体验。本文介绍了 <b>Eino</b> 的特点、组成、使用方法和示例，帮助 Go 开发者在 2025 年打通 LLM 应用开发。</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://www.cloudwego.io/docs/eino/">Eino: User Manual | CloudWeGo</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://www.cloudwego.io/docs/eino/">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/www.cloudwego.io.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://www.cloudwego.io/docs/eino/">
                  www.cloudwego.io/docs/eino/
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-06-20T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://www.cloudwego.io/docs/eino/"><b>Eino</b> aims to provide an AI application development framework built with Go. <b>Eino</b> refers to many excellent AI application development frameworks in the open-source community, such as LangChain, LangGraph, LlamaIndex, etc., and provides an AI application development framework that is more in line with the programming habits of Golang.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://pkg.go.dev/github.com/cloudwego/eino">eino package - github.com/cloudwego/eino - Go Packages</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://pkg.go.dev/github.com/cloudwego/eino">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/pkg.go.dev.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://pkg.go.dev/github.com/cloudwego/eino">
                  pkg.go.dev/github.com/cloudwego/eino
                  </a>

                  
                    <span>&nbsp; &nbsp; 2025-06-19T00:00:00.0000000</span>
                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://pkg.go.dev/github.com/cloudwego/eino"><b>Eino</b> is a Golang package that aims to be the ultimate LLM application development framework. It provides component abstractions, orchestration APIs, best practices, tools and examples for building LLM applications.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://github.com/cloudwego/eino-ext">GitHub - cloudwego/eino-ext: Various extensions for the Eino framework ...</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://github.com/cloudwego/eino-ext">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/github.com.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://github.com/cloudwego/eino-ext">
                  github.com/cloudwego/eino-ext
                  </a>

                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://github.com/cloudwego/eino-ext"><b>Eino</b>-ext is a project that hosts various extensions for the <b>Eino</b> framework, a powerful and flexible framework for building LLM applications. The extensions include component implementations, callback handlers, and devops tools for <b>Eino</b>.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="https://en.wikipedia.org/wiki/Eino">Eino - Wikipedia</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="https://en.wikipedia.org/wiki/Eino">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/en.wikipedia.org.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="https://en.wikipedia.org/wiki/Eino">
                  en.wikipedia.org/wiki/Eino
                  </a>

                  

                </div>
            </div>

            
                  <a class="result__snippet" href="https://en.wikipedia.org/wiki/Eino"><b>Eino</b> is a Finnish and Estonian masculine given name derived from Henri or Enewald. It is also the name of many notable people in various fields, such as sports, politics, arts and science.</a>
            

            <div class="clear"></div>
          </div>

        </div>

  




        
        
                <div class="nav-link">
        <form action="/html/" method="post">
          <input type="submit" class='btn btn--alt' value="Next" />
          <input type="hidden" name="q" value="eino" />
          <input type="hidden" name="s" value="10" />
          <input type="hidden" name="nextParams" value="" />
          <input type="hidden" name="v" value="l" />
          <input type="hidden" name="o" value="json" />
          <input type="hidden" name="dc" value="11" />
          <input type="hidden" name="api" value="d.js" />
          <input type="hidden" name="vqd" value="4-167208798460794982980732831674372650598" />

        
        
        
          <input name="kl" value="wt-wt" type="hidden" />
        
        
        
        
        </form>
                </div>
        



        <div class=" feedback-btn">
            <a rel="nofollow" href="//duckduckgo.com/feedback.html" target="_new">Feedback</a>
        </div>
        <div class="clear"></div>
  </div>
  </div> <!-- links wrapper //-->



    </div>
  </div>

    <div id="bottom_spacing2"></div>

    
      <img src="//duckduckgo.com/t/sl_h"/>
    
</body>
</html>
`

		results, nextReqBody, err := parseTextHTMLSearchResponse(respBody)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(results))
		assert.NotEmpty(t, nextReqBody["vqd"])
	})

	PatchConvey("no results", t, func() {
		respBodyEOF := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">

<!--[if IE 6]><html class="ie6" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 7]><html class="lt-ie8 lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 8]><html class="lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if gt IE 8]><!--><html xmlns="http://www.w3.org/1999/xhtml"><!--<![endif]-->
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=3.0, user-scalable=1" />
  <meta name="referrer" content="origin" />
  <meta name="HandheldFriendly" content="true" />
  <meta name="robots" content="noindex, nofollow" />
  <title>testtesttesttesttesttesttesttesttesttesttest at DuckDuckGo</title>
  <link title="DuckDuckGo (HTML)" type="application/opensearchdescription+xml" rel="search" href="//duckduckgo.com/opensearch_html_v2.xml" />
  <link href="//duckduckgo.com/favicon.ico" rel="shortcut icon" />
  <link rel="icon" href="//duckduckgo.com/favicon.ico" type="image/x-icon" />
  <link id="icon60" rel="apple-touch-icon" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_60x60.png?v=2"/>
  <link id="icon76" rel="apple-touch-icon" sizes="76x76" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_76x76.png?v=2"/>
  <link id="icon120" rel="apple-touch-icon" sizes="120x120" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_120x120.png?v=2"/>
  <link id="icon152" rel="apple-touch-icon" sizes="152x152" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_152x152.png?v=2"/>
  <link rel="image_src" href="//duckduckgo.com/assets/icons/meta/DDG-icon_256x256.png">
  <link rel="stylesheet" media="handheld, all" href="//duckduckgo.com/dist/h.0d389dd02de7e1030cde.css" type="text/css"/>
</head>

<body class="body--html">
  <a name="top" id="top"></a>

  <form action="/html/" method="post">
    <input type="text" name="state_hidden" id="state_hidden" />
  </form>

  <div>
    <div class="site-wrapper-border"></div>

    <div id="header" class="header cw header--html">
        <a title="DuckDuckGo" href="/html/" class="header__logo-wrap"></a>


    <form name="x" class="header__form" action="/html/" method="post">

      <div class="search search--header">
          <input name="q" autocomplete="off" class="search__input" id="search_form_input_homepage" type="text" value="testtesttesttesttesttesttesttesttesttesttest" />
          <input name="b" id="search_button_homepage" class="search__button search__button--html" value="" title="Search" alt="Search" type="submit" />
      </div>


    
    
    
    

    <div class="frm__select">
      <select name="kl">
      
        <option value="" >All Regions</option>
      
        <option value="ar-es" >Argentina</option>
      
        <option value="au-en" >Australia</option>
      
        <option value="at-de" >Austria</option>
      
        <option value="be-fr" >Belgium (fr)</option>
      
        <option value="be-nl" >Belgium (nl)</option>
      
        <option value="br-pt" >Brazil</option>
      
        <option value="bg-bg" >Bulgaria</option>
      
        <option value="ca-en" >Canada (en)</option>
      
        <option value="ca-fr" >Canada (fr)</option>
      
        <option value="ct-ca" >Catalonia</option>
      
        <option value="cl-es" >Chile</option>
      
        <option value="cn-zh" selected>China</option>
      
        <option value="co-es" >Colombia</option>
      
        <option value="hr-hr" >Croatia</option>
      
        <option value="cz-cs" >Czech Republic</option>
      
        <option value="dk-da" >Denmark</option>
      
        <option value="ee-et" >Estonia</option>
      
        <option value="fi-fi" >Finland</option>
      
        <option value="fr-fr" >France</option>
      
        <option value="de-de" >Germany</option>
      
        <option value="gr-el" >Greece</option>
      
        <option value="hk-tzh" >Hong Kong</option>
      
        <option value="hu-hu" >Hungary</option>
      
        <option value="is-is" >Iceland</option>
      
        <option value="in-en" >India (en)</option>
      
        <option value="id-en" >Indonesia (en)</option>
      
        <option value="ie-en" >Ireland</option>
      
        <option value="il-en" >Israel (en)</option>
      
        <option value="it-it" >Italy</option>
      
        <option value="jp-jp" >Japan</option>
      
        <option value="kr-kr" >Korea</option>
      
        <option value="lv-lv" >Latvia</option>
      
        <option value="lt-lt" >Lithuania</option>
      
        <option value="my-en" >Malaysia (en)</option>
      
        <option value="mx-es" >Mexico</option>
      
        <option value="nl-nl" >Netherlands</option>
      
        <option value="nz-en" >New Zealand</option>
      
        <option value="no-no" >Norway</option>
      
        <option value="pk-en" >Pakistan (en)</option>
      
        <option value="pe-es" >Peru</option>
      
        <option value="ph-en" >Philippines (en)</option>
      
        <option value="pl-pl" >Poland</option>
      
        <option value="pt-pt" >Portugal</option>
      
        <option value="ro-ro" >Romania</option>
      
        <option value="ru-ru" >Russia</option>
      
        <option value="xa-ar" >Saudi Arabia</option>
      
        <option value="sg-en" >Singapore</option>
      
        <option value="sk-sk" >Slovakia</option>
      
        <option value="sl-sl" >Slovenia</option>
      
        <option value="za-en" >South Africa</option>
      
        <option value="es-ca" >Spain (ca)</option>
      
        <option value="es-es" >Spain (es)</option>
      
        <option value="se-sv" >Sweden</option>
      
        <option value="ch-de" >Switzerland (de)</option>
      
        <option value="ch-fr" >Switzerland (fr)</option>
      
        <option value="tw-tzh" >Taiwan</option>
      
        <option value="th-en" >Thailand (en)</option>
      
        <option value="tr-tr" >Turkey</option>
      
        <option value="us-en" >US (English)</option>
      
        <option value="us-es" >US (Spanish)</option>
      
        <option value="ua-uk" >Ukraine</option>
      
        <option value="uk-en" >United Kingdom</option>
      
        <option value="vn-en" >Vietnam (en)</option>
      
      </select>
    </div>

    <div class="frm__select frm__select--last">
      <select class="" name="df">
      
        <option value="" >Any Time</option>
      
        <option value="d" selected>Past Day</option>
      
        <option value="w" >Past Week</option>
      
        <option value="m" >Past Month</option>
      
        <option value="y" >Past Year</option>
      
      </select>
    </div>

    </form>

    </div>





<!-- Web results are present -->

  <div>
  <div class="serp__results">
  <div id="links" class="results">

      



  


            <div class="result results_links results_links_deep web-result ">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
            <a rel="nofollow" class="result__a" href="http://www.google.com/search?hl=en&amp;q=testtesttesttesttesttesttesttesttesttesttest">EOF</a>
          
          </h2>

      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                      <a rel="nofollow" href="http://www.google.com/search?hl=en&amp;q=testtesttesttesttesttesttesttesttesttesttest">
                        <img class="result__icon__img" width="16" height="16" alt=""
                          src="//external-content.duckduckgo.com/ip3/www.google.com.ico" name="i15" />
                      </a>
                  
                  </span>

                  <a class="result__url" href="http://www.google.com/search?hl=en&amp;q=testtesttesttesttesttesttesttesttesttesttest">
                  google.com search
                  </a>

                  

                </div>
            </div>

            

            <div class="clear"></div>
          </div>

        </div>

  



  


            <div class="result results_links results_links_deep web-result result--no-result">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
          </h2>

      
            <div class="no-results"></div>
      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                  </span>

                  <a class="result__url" href="">
                  
                  </a>

                  

                </div>
            </div>

            

            <div class="clear"></div>
          </div>

        </div>

  





        <div class=" feedback-btn">
            <a rel="nofollow" href="//duckduckgo.com/feedback.html" target="_new">Feedback</a>
        </div>
        <div class="clear"></div>
  </div>
  </div> <!-- links wrapper //-->



    </div>
  </div>

    <div id="bottom_spacing2"></div>

    
      <img src="//duckduckgo.com/t/sl_h"/>
    
</body>
</html>
`
		respBodyNoResults := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">

<!--[if IE 6]><html class="ie6" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 7]><html class="lt-ie8 lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if IE 8]><html class="lt-ie9" xmlns="http://www.w3.org/1999/xhtml"><![endif]-->
<!--[if gt IE 8]><!--><html xmlns="http://www.w3.org/1999/xhtml"><!--<![endif]-->
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=3.0, user-scalable=1" />
  <meta name="referrer" content="origin" />
  <meta name="HandheldFriendly" content="true" />
  <meta name="robots" content="noindex, nofollow" />
  <title>eino at DuckDuckGo</title>
  <link title="DuckDuckGo (HTML)" type="application/opensearchdescription+xml" rel="search" href="//duckduckgo.com/opensearch_html_v2.xml" />
  <link href="//duckduckgo.com/favicon.ico" rel="shortcut icon" />
  <link rel="icon" href="//duckduckgo.com/favicon.ico" type="image/x-icon" />
  <link id="icon60" rel="apple-touch-icon" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_60x60.png?v=2"/>
  <link id="icon76" rel="apple-touch-icon" sizes="76x76" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_76x76.png?v=2"/>
  <link id="icon120" rel="apple-touch-icon" sizes="120x120" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_120x120.png?v=2"/>
  <link id="icon152" rel="apple-touch-icon" sizes="152x152" href="//duckduckgo.com/assets/icons/meta/DDG-iOS-icon_152x152.png?v=2"/>
  <link rel="image_src" href="//duckduckgo.com/assets/icons/meta/DDG-icon_256x256.png">
  <link rel="stylesheet" media="handheld, all" href="//duckduckgo.com/dist/h.0d389dd02de7e1030cde.css" type="text/css"/>
</head>

<body class="body--html">
  <a name="top" id="top"></a>

  <form action="/html/" method="post">
    <input type="text" name="state_hidden" id="state_hidden" />
  </form>

  <div>
    <div class="site-wrapper-border"></div>

    <div id="header" class="header cw header--html">
        <a title="DuckDuckGo" href="/html/" class="header__logo-wrap"></a>


    <form name="x" class="header__form" action="/html/" method="post">

      <div class="search search--header">
          <input name="q" autocomplete="off" class="search__input" id="search_form_input_homepage" type="text" value="eino" />
          <input name="b" id="search_button_homepage" class="search__button search__button--html" value="" title="Search" alt="Search" type="submit" />
      </div>


    
    
    
    

    <div class="frm__select">
      <select name="kl">
      
        <option value="" >All Regions</option>
      
        <option value="ar-es" >Argentina</option>
      
        <option value="au-en" >Australia</option>
      
        <option value="at-de" >Austria</option>
      
        <option value="be-fr" >Belgium (fr)</option>
      
        <option value="be-nl" >Belgium (nl)</option>
      
        <option value="br-pt" >Brazil</option>
      
        <option value="bg-bg" >Bulgaria</option>
      
        <option value="ca-en" >Canada (en)</option>
      
        <option value="ca-fr" >Canada (fr)</option>
      
        <option value="ct-ca" >Catalonia</option>
      
        <option value="cl-es" >Chile</option>
      
        <option value="cn-zh" >China</option>
      
        <option value="co-es" >Colombia</option>
      
        <option value="hr-hr" >Croatia</option>
      
        <option value="cz-cs" >Czech Republic</option>
      
        <option value="dk-da" >Denmark</option>
      
        <option value="ee-et" >Estonia</option>
      
        <option value="fi-fi" >Finland</option>
      
        <option value="fr-fr" >France</option>
      
        <option value="de-de" >Germany</option>
      
        <option value="gr-el" >Greece</option>
      
        <option value="hk-tzh" >Hong Kong</option>
      
        <option value="hu-hu" >Hungary</option>
      
        <option value="is-is" >Iceland</option>
      
        <option value="in-en" >India (en)</option>
      
        <option value="id-en" >Indonesia (en)</option>
      
        <option value="ie-en" >Ireland</option>
      
        <option value="il-en" >Israel (en)</option>
      
        <option value="it-it" >Italy</option>
      
        <option value="jp-jp" >Japan</option>
      
        <option value="kr-kr" >Korea</option>
      
        <option value="lv-lv" >Latvia</option>
      
        <option value="lt-lt" >Lithuania</option>
      
        <option value="my-en" >Malaysia (en)</option>
      
        <option value="mx-es" >Mexico</option>
      
        <option value="nl-nl" >Netherlands</option>
      
        <option value="nz-en" >New Zealand</option>
      
        <option value="no-no" >Norway</option>
      
        <option value="pk-en" >Pakistan (en)</option>
      
        <option value="pe-es" >Peru</option>
      
        <option value="ph-en" >Philippines (en)</option>
      
        <option value="pl-pl" >Poland</option>
      
        <option value="pt-pt" >Portugal</option>
      
        <option value="ro-ro" >Romania</option>
      
        <option value="ru-ru" >Russia</option>
      
        <option value="xa-ar" >Saudi Arabia</option>
      
        <option value="sg-en" >Singapore</option>
      
        <option value="sk-sk" >Slovakia</option>
      
        <option value="sl-sl" >Slovenia</option>
      
        <option value="za-en" >South Africa</option>
      
        <option value="es-ca" >Spain (ca)</option>
      
        <option value="es-es" >Spain (es)</option>
      
        <option value="se-sv" >Sweden</option>
      
        <option value="ch-de" >Switzerland (de)</option>
      
        <option value="ch-fr" >Switzerland (fr)</option>
      
        <option value="tw-tzh" >Taiwan</option>
      
        <option value="th-en" >Thailand (en)</option>
      
        <option value="tr-tr" >Turkey</option>
      
        <option value="us-en" >US (English)</option>
      
        <option value="us-es" >US (Spanish)</option>
      
        <option value="ua-uk" >Ukraine</option>
      
        <option value="uk-en" >United Kingdom</option>
      
        <option value="vn-en" >Vietnam (en)</option>
      
      </select>
    </div>

    <div class="frm__select frm__select--last">
      <select class="" name="df">
      
        <option value="" >Any Time</option>
      
        <option value="d" >Past Day</option>
      
        <option value="w" selected>Past Week</option>
      
        <option value="m" >Past Month</option>
      
        <option value="y" >Past Year</option>
      
      </select>
    </div>

    </form>

    </div>





<!-- Web results are present -->

  <div>
  <div class="serp__results">
  <div id="links" class="results">

      



  


            <div class="result results_links results_links_deep web-result result--no-result">


          <div class="links_main links_deep result__body"> <!-- This is the visible part -->

          <h2 class="result__title">
          
          </h2>

      
            <div class="no-results">No more results.</div>
      

            <div class="result__extras">
                <div class="result__extras__url">
                  <span class="result__icon">
                    
                  </span>

                  <a class="result__url" href="">
                  
                  </a>

                  

                </div>
            </div>

            

            <div class="clear"></div>
          </div>

        </div>

  





        <div class=" feedback-btn">
            <a rel="nofollow" href="//duckduckgo.com/feedback.html" target="_new">Feedback</a>
        </div>
        <div class="clear"></div>
  </div>
  </div> <!-- links wrapper //-->



    </div>
  </div>

    <div id="bottom_spacing2"></div>

    
      <img src="//duckduckgo.com/t/sl_h"/>
    
</body>
</html>
`

		results, _, err := parseTextHTMLSearchResponse(respBodyEOF)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(results))
		results, _, err = parseTextHTMLSearchResponse(respBodyNoResults)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})
}
