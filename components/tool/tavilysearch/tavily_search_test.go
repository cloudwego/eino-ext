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

package tavilysearch

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
)

func TestTavilySearchTool(t *testing.T) {
	const mockSearchQuery = "What is transformer?"
	const mockSearchResult = `
{
  "query": "what is transformer",
  "follow_up_questions": null,
  "answer": null,
  "images": [],
  "results": [
    {
      "title": "Transformer: What is it? (Definition And Working Principle)",
      "url": "https://www.electrical4u.com/what-is-transformer-definition-working-principle-of-transformer/",
      "content": "A transformer is defined as a passive electrical device that transfers electrical energy from one circuit to another through the process of electromagnetic induction. It is most commonly used to increase ('step up') or decrease ('step down') voltage levels between circuits.",
      "score": 0.9006087,
      "raw_content": null
    },
    {
      "title": "Transformer | Definition, Types, & Facts | Britannica",
      "url": "https://www.britannica.com/technology/transformer-electronics",
      "content": "Transformer, device that transfers electric energy from one alternating-current circuit to one or more other circuits, either increasing (stepping up) or reducing (stepping down) the voltage. Transformers are employed for widely varying purposes. Learn more about transformers in this article.",
      "score": 0.8440111,
      "raw_content": null
    },
    {
      "title": "How do electricity transformers work? - Explain that Stuff",
      "url": "https://www.explainthatstuff.com/transformers.html",
      "content": "How does a transformer work? A transformer is based on a very simple fact about electricity: when a fluctuating electric current flows through a wire, it generates a magnetic field (an invisible pattern of magnetism) or \"magnetic flux\" all around it. The strength of the magnetism (which has the rather technical name of magnetic flux density) is directly related to the size of the electric",
      "score": 0.6364215,
      "raw_content": null
    },
    {
      "title": "Transformer - Wikipedia",
      "url": "https://en.wikipedia.org/wiki/Transformer",
      "content": "[Jump to content](https://en.wikipedia.org/wiki/Transformer#bodyContent) *   [Random article](https://en.wikipedia.org/wiki/Special:Random \"Visit a randomly selected article [x]\") [Search](https://en.wikipedia.org/wiki/Special:Search \"Search Wikipedia [f]\") *   [Contributions](https://en.wikipedia.org/wiki/Special:MyContributions \"A list of edits made from this IP address [y]\") *   [(Top)](https://en.wikipedia.org/wiki/Transformer#) *   [1 Principles](https://en.wikipedia.org/wiki/Transformer#Principles)Toggle Principles subsection *   [1.1 Ideal transformer](https://en.wikipedia.org/wiki/Transformer#Ideal_transformer) *   [1.2.3 Equivalent circuit](https://en.wikipedia.org/wiki/Transformer#Equivalent_circuit) *   [1.4 Polarity](https://en.wikipedia.org/wiki/Transformer#Polarity) *   [1.5 Effect of frequency](https://en.wikipedia.org/wiki/Transformer#Effect_of_frequency) *   [2 Construction](https://en.wikipedia.org/wiki/Transformer#Construction)Toggle Construction subsection *   [2.1 Cores](https://en.wikipedia.org/wiki/Transformer#Cores) *   [2.1.2 Solid cores](https://en.wikipedia.org/wiki/Transformer#Solid_cores) *   [2.1.3 Toroidal cores](https://en.wikipedia.org/wiki/Transformer#Toroidal_cores) *   [2.1.4 Air cores](https://en.wikipedia.org/wiki/Transformer#Air_cores) *   [2.2 Windings](https://en.wikipedia.org/wiki/Transformer#Windings) *   [2.3 Cooling](https://en.wikipedia.org/wiki/Transformer#Cooling) *   [2.4 Insulation](https://en.wikipedia.org/wiki/Transformer#Insulation) *   [2.5 Bushings](https://en.wikipedia.org/wiki/Transformer#Bushings) *   [4 Applications](https://en.wikipedia.org/wiki/Transformer#Applications) *   [5 History](https://en.wikipedia.org/wiki/Transformer#History)Toggle History subsection *   [5.1 Discovery of induction](https://en.wikipedia.org/wiki/Transformer#Discovery_of_induction) *   [5.2 Induction coils](https://en.wikipedia.org/wiki/Transformer#Induction_coils) *   [6 See also](https://en.wikipedia.org/wiki/Transformer#See_also) *   [7 Notes](https://en.wikipedia.org/wiki/Transformer#Notes) *   [8 References](https://en.wikipedia.org/wiki/Transformer#References) *   [9 Bibliography](https://en.wikipedia.org/wiki/Transformer#Bibliography) *   [10 External links](https://en.wikipedia.org/wiki/Transformer#External_links) 112 languages[Add topic](https://en.wikipedia.org/wiki/Transformer#)",
      "score": 0.57637566,
      "raw_content": null
    },
    {
      "title": "What is a Transformer ? Construction, Working, Types & Uses",
      "url": "https://www.electricaltechnology.org/2012/02/working-principle-of-transformer.html",
      "content": "Learn what is an electrical transformer, how it works on the principle of mutual induction, and what are its parts, types and applications. Find out the difference between ideal and practical transformers, their equivalent circuit, EMF equation, losses and efficiency.",
      "score": 0.47928494,
      "raw_content": null
    }
  ],
  "response_time": 1.58
}`
	const expectedSchema = `
{
	"type": "object",
	"properties": {
	  "query": {
		"description": "The search query to execute with Tavily.",
		"type": "string"
	  },
	  "topic": {
		"description": "The category of the search. general or news.",
		"type": "string"
	  }
	},
	"required": [
	  "query"
	]
}
`
	const expectedOutput = "{\"query\":\"what is transformer\",\"answer\":\"\",\"results\":[{\"title\":\"Transformer: What is it? (Definition And Working Principle)\",\"url\":\"https://www.electrical4u.com/what-is-transformer-definition-working-principle-of-transformer/\",\"content\":\"A transformer is defined as a passive electrical device that transfers electrical energy from one circuit to another through the process of electromagnetic induction. It is most commonly used to increase ('step up') or decrease ('step down') voltage levels between circuits.\",\"score\":0.9006087,\"raw_content\":\"\"},{\"title\":\"Transformer | Definition, Types, & Facts | Britannica\",\"url\":\"https://www.britannica.com/technology/transformer-electronics\",\"content\":\"Transformer, device that transfers electric energy from one alternating-current circuit to one or more other circuits, either increasing (stepping up) or reducing (stepping down) the voltage. Transformers are employed for widely varying purposes. Learn more about transformers in this article.\",\"score\":0.8440111,\"raw_content\":\"\"},{\"title\":\"How do electricity transformers work? - Explain that Stuff\",\"url\":\"https://www.explainthatstuff.com/transformers.html\",\"content\":\"How does a transformer work? A transformer is based on a very simple fact about electricity: when a fluctuating electric current flows through a wire, it generates a magnetic field (an invisible pattern of magnetism) or \\\"magnetic flux\\\" all around it. The strength of the magnetism (which has the rather technical name of magnetic flux density) is directly related to the size of the electric\",\"score\":0.6364215,\"raw_content\":\"\"},{\"title\":\"Transformer - Wikipedia\",\"url\":\"https://en.wikipedia.org/wiki/Transformer\",\"content\":\"[Jump to content](https://en.wikipedia.org/wiki/Transformer#bodyContent) *   [Random article](https://en.wikipedia.org/wiki/Special:Random \\\"Visit a randomly selected article [x]\\\") [Search](https://en.wikipedia.org/wiki/Special:Search \\\"Search Wikipedia [f]\\\") *   [Contributions](https://en.wikipedia.org/wiki/Special:MyContributions \\\"A list of edits made from this IP address [y]\\\") *   [(Top)](https://en.wikipedia.org/wiki/Transformer#) *   [1 Principles](https://en.wikipedia.org/wiki/Transformer#Principles)Toggle Principles subsection *   [1.1 Ideal transformer](https://en.wikipedia.org/wiki/Transformer#Ideal_transformer) *   [1.2.3 Equivalent circuit](https://en.wikipedia.org/wiki/Transformer#Equivalent_circuit) *   [1.4 Polarity](https://en.wikipedia.org/wiki/Transformer#Polarity) *   [1.5 Effect of frequency](https://en.wikipedia.org/wiki/Transformer#Effect_of_frequency) *   [2 Construction](https://en.wikipedia.org/wiki/Transformer#Construction)Toggle Construction subsection *   [2.1 Cores](https://en.wikipedia.org/wiki/Transformer#Cores) *   [2.1.2 Solid cores](https://en.wikipedia.org/wiki/Transformer#Solid_cores) *   [2.1.3 Toroidal cores](https://en.wikipedia.org/wiki/Transformer#Toroidal_cores) *   [2.1.4 Air cores](https://en.wikipedia.org/wiki/Transformer#Air_cores) *   [2.2 Windings](https://en.wikipedia.org/wiki/Transformer#Windings) *   [2.3 Cooling](https://en.wikipedia.org/wiki/Transformer#Cooling) *   [2.4 Insulation](https://en.wikipedia.org/wiki/Transformer#Insulation) *   [2.5 Bushings](https://en.wikipedia.org/wiki/Transformer#Bushings) *   [4 Applications](https://en.wikipedia.org/wiki/Transformer#Applications) *   [5 History](https://en.wikipedia.org/wiki/Transformer#History)Toggle History subsection *   [5.1 Discovery of induction](https://en.wikipedia.org/wiki/Transformer#Discovery_of_induction) *   [5.2 Induction coils](https://en.wikipedia.org/wiki/Transformer#Induction_coils) *   [6 See also](https://en.wikipedia.org/wiki/Transformer#See_also) *   [7 Notes](https://en.wikipedia.org/wiki/Transformer#Notes) *   [8 References](https://en.wikipedia.org/wiki/Transformer#References) *   [9 Bibliography](https://en.wikipedia.org/wiki/Transformer#Bibliography) *   [10 External links](https://en.wikipedia.org/wiki/Transformer#External_links) 112 languages[Add topic](https://en.wikipedia.org/wiki/Transformer#)\",\"score\":0.57637566,\"raw_content\":\"\"},{\"title\":\"What is a Transformer ? Construction, Working, Types & Uses\",\"url\":\"https://www.electricaltechnology.org/2012/02/working-principle-of-transformer.html\",\"content\":\"Learn what is an electrical transformer, how it works on the principle of mutual induction, and what are its parts, types and applications. Find out the difference between ideal and practical transformers, their equivalent circuit, EMF equation, losses and efficiency.\",\"score\":0.47928494,\"raw_content\":\"\"}],\"images\":[]}"

	searchResponse := &SearchResponse{}
	err := sonic.UnmarshalString(mockSearchResult, searchResponse)
	assert.NoError(t, err)

	mockey.PatchConvey("TestTavilySearchTool", t, func() {
		ctx := context.Background()
		mockResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(mockSearchResult)),
			Header:     http.Header{},
		}
		mockey.Mock((*http.Client).Do).Return(mockResp, nil).Build()
		conf := &Config{
			APIKey: "{mock_api_key}",
		}

		st, err := NewTool(ctx, conf)
		assert.NoError(t, err)

		tl, err := st.Info(ctx)
		assert.NoError(t, err)

		js, err := tl.ToOpenAPIV3()
		assert.NoError(t, err)
		body, err := js.MarshalJSON()
		assert.NoError(t, err)

		assert.JSONEq(t, expectedSchema, string(body))

		tsReq := &SearchRequest{
			Query: mockSearchQuery,
		}

		tsBody, err := sonic.MarshalString(tsReq)
		assert.NoError(t, err)

		toolOut, err := st.InvokableRun(ctx, tsBody)
		assert.NoError(t, err)

		assert.Equal(t, expectedOutput, toolOut)
	})
}
