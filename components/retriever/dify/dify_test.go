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

package dify

import (
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
)

func TestRecord_ToDoc(t *testing.T) {
	PatchConvey("Test Record.toDoc", t, func() {
		PatchConvey("When record is valid", func() {
			record := &Record{
				Segment: &Segment{
					ID:         "1",
					Content:    "test content",
					DocumentID: "doc1",
					Document: &Document{
						ID:             "1",
						DataSourceType: "markdown",
						Name:           "test.md",
					},
				},
				Score: 0.8,
			}
			expected := &schema.Document{
				ID:      "1",
				Content: "test content",
				MetaData: map[string]interface{}{
					origDocIDKey:   "doc1",
					origDocNameKey: "test.md",
				},
			}

			result := record.toDoc()
			convey.So(result.ID, convey.ShouldEqual, expected.ID)
			convey.So(result.Content, convey.ShouldEqual, expected.Content)
			convey.So(result.MetaData[origDocIDKey], convey.ShouldEqual, expected.MetaData[origDocIDKey])
		})

		PatchConvey("When record is nil", func() {
			var record *Record
			result := record.toDoc()
			convey.So(result, convey.ShouldBeNil)
		})

		PatchConvey("When segment is nil", func() {
			record := &Record{
				Segment: nil,
				Score:   0.8,
			}
			result := record.toDoc()
			convey.So(result, convey.ShouldBeNil)
		})
	})
}

func TestMetadataFunctions(t *testing.T) {
	PatchConvey("Test metadata functions", t, func() {
		PatchConvey("When document is not nil", func() {
			doc := &schema.Document{
				MetaData: map[string]interface{}{},
			}

			PatchConvey("Test OrgDocID functions", func() {
				setOrgDocID(doc, "doc1")
				convey.So(GetOrgDocID(doc), convey.ShouldEqual, "doc1")
			})

			PatchConvey("Test OrgDocName functions", func() {
				setOrgDocName(doc, "test doc")
				convey.So(GetOrgDocName(doc), convey.ShouldEqual, "test doc")
			})

			PatchConvey("Test Keywords functions", func() {
				keywords := []string{"test", "keywords"}
				setKeywords(doc, keywords)
				convey.So(GetKeywords(doc), convey.ShouldResemble, keywords)
			})
		})

		PatchConvey("When document is nil", func() {
			var nilDoc *schema.Document
			setOrgDocID(nilDoc, "doc1")
			setOrgDocName(nilDoc, "test doc")
			setKeywords(nilDoc, []string{"test", "keywords"})
			convey.So(GetOrgDocID(nilDoc), convey.ShouldEqual, "")
			convey.So(GetOrgDocName(nilDoc), convey.ShouldEqual, "")
			convey.So(GetKeywords(nilDoc), convey.ShouldBeNil)
		})
	})
}

func TestRetrievalModel_Copy(t *testing.T) {
	PatchConvey("Test RetrievalModel.copy", t, func() {
		PatchConvey("When model is nil", func() {
			var model *RetrievalModel
			result := model.copy()
			convey.So(result, convey.ShouldBeNil)
		})

		PatchConvey("When model has complete configuration", func() {
			model := &RetrievalModel{
				SearchMethod:          SearchMethodSemantic,
				RerankingEnable:       ptrOf(true),
				RerankingMode:         ptrOf("hybrid"),
				Weights:               ptrOf(0.7),
				TopK:                  ptrOf(10),
				ScoreThreshold:        ptrOf(0.8),
				ScoreThresholdEnabled: ptrOf(true),
				RerankingModel: &RerankingModel{
					RerankingProviderName: "openai",
					RerankingModelName:    "gpt-3.5-turbo",
				},
			}

			result := model.copy()
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.SearchMethod, convey.ShouldEqual, SearchMethodSemantic)
			convey.So(*result.RerankingEnable, convey.ShouldBeTrue)
			convey.So(*result.RerankingMode, convey.ShouldEqual, "hybrid")
			convey.So(*result.Weights, convey.ShouldEqual, 0.7)
			convey.So(*result.TopK, convey.ShouldEqual, 10)
			convey.So(*result.ScoreThreshold, convey.ShouldEqual, 0.8)
			convey.So(*result.ScoreThresholdEnabled, convey.ShouldBeTrue)
			convey.So(result.RerankingModel, convey.ShouldNotBeNil)
			convey.So(result.RerankingModel.RerankingProviderName, convey.ShouldEqual, "openai")
			convey.So(result.RerankingModel.RerankingModelName, convey.ShouldEqual, "gpt-3.5-turbo")
		})

		PatchConvey("When model has partial configuration", func() {
			model := &RetrievalModel{
				SearchMethod:   SearchMethodKeyword,
				TopK:           ptrOf(5),
				RerankingModel: nil,
			}

			result := model.copy()
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.SearchMethod, convey.ShouldEqual, SearchMethodKeyword)
			convey.So(*result.TopK, convey.ShouldEqual, 5)
			convey.So(result.RerankingEnable, convey.ShouldBeNil)
			convey.So(result.RerankingMode, convey.ShouldBeNil)
			convey.So(result.Weights, convey.ShouldBeNil)
			convey.So(result.ScoreThreshold, convey.ShouldBeNil)
			convey.So(result.ScoreThresholdEnabled, convey.ShouldBeNil)
			convey.So(result.RerankingModel, convey.ShouldBeNil)
		})

		PatchConvey("test RetrievalModel change", func() {
			model := &RetrievalModel{
				SearchMethod:   SearchMethodKeyword,
				TopK:           ptrOf(5),
				RerankingModel: nil,
			}

			result := model.copy()
			result.TopK = ptrOf(10)
			convey.So(*model.TopK, convey.ShouldEqual, 5)
		})
	})
}
